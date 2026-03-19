/**
 * Copyright 2023 Google LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React, { KeyboardEvent, useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { QueryEditorProps, SelectableValue } from '@grafana/data';
import { Alert, InlineField, InlineFieldRow, AsyncSelect, Input, LinkButton, RadioButtonGroup, TextArea, Tooltip } from '@grafana/ui';
import { DataSource } from './datasource';
import { CloudTraceOptions, defaultQuery, Query } from './types';

type Props = QueryEditorProps<DataSource, Query, CloudTraceOptions>;

/**
 * This is basically copied from {MQLQueryEditor} from the cloud-monitoring data source
 *
 */
export function CloudTraceQueryEditor({ datasource, query, range, onChange, onRunQuery }: React.PropsWithChildren<Props>) {
  const onKeyDownTextArea = (event: KeyboardEvent<HTMLTextAreaElement>) => {
    if (event.key === 'Enter' && (event.shiftKey || event.ctrlKey)) {
      event.preventDefault();
      onRunQuery();
    }
  };

  const onKeyDownInput = (event: KeyboardEvent<HTMLInputElement>) => {
    if (event.key === 'Enter' && (event.shiftKey || event.ctrlKey)) {
      event.preventDefault();
      onRunQuery();
    }
  };

  const [fetchError, setFetchError] = useState<string | undefined>();
  const requestIdRef = useRef(0);

  /**
   * Sanitize fetch errors — Grafana's backendSrv may include raw HTML bodies
   * from proxy/universe-domain errors in err.data or err.message.
   */
  const sanitizeFetchError = (err: unknown): string => {
    const errData = (err as any)?.data;
    // When the backend returns JSON { "message": "..." }, err.data is the parsed object
    const raw = (typeof errData === 'object' && errData?.message)
      ? errData.message
      : errData ?? (err as any)?.message ?? String(err);
    const text = typeof raw === 'string' ? raw : JSON.stringify(raw);
    // Detect HTML content (full page or gRPC content-type error)
    if (/<html[\s>]|<!doctype\s+html/i.test(text) || text.includes('text/html')) {
      return 'The server returned an HTML error page. If you have configured a Universe Domain, please verify it is correct.';
    }
    return text;
  };

  const loadProjects = useCallback((inputValue: string): Promise<Array<SelectableValue<string>>> => {
    const thisRequestId = ++requestIdRef.current;
    return datasource.getProjects(inputValue || undefined).then(res => {
      if (thisRequestId === requestIdRef.current) {
        setFetchError(undefined);
      }
      return res.map(project => ({
        label: project,
        value: project,
      }));
    }).catch(err => {
      if (thisRequestId === requestIdRef.current) {
        setFetchError(sanitizeFetchError(err));
      }
      return [];
    });
  }, [datasource]);


  // Apply defaults if needed — use onChange so they are persisted in the panel config
  useEffect(() => {
    const needsQueryText = query.queryText == null && defaultQuery.queryText;
    const needsProjectId = !query.projectId;

    if (!needsQueryText && !needsProjectId) {
      return;
    }

    if (needsProjectId) {
      datasource.getDefaultProject().then((project) => {
        const nextQuery = { ...query };
        if (needsQueryText) {
          nextQuery.queryText = defaultQuery.queryText;
        }
        if (project) {
          nextQuery.projectId = project;
        }
        onChange(nextQuery);
      });
    } else if (needsQueryText) {
      onChange({ ...query, queryText: defaultQuery.queryText });
    }
  }, [query, datasource, onChange]);

  /**
   * Keep an up-to-date URI that links to the equivalent query in the GCP console
   */
  const gcpConsoleURI = useMemo<string | undefined>(() => {
    const timeRangeParam = range !== undefined ?
      `&start=${range?.from?.valueOf()}&end=${range?.to?.valueOf()}`
      : '';
    const projectParam = query.projectId !== undefined ?
      `&project=${query.projectId}`
      : '';
    const filterParam = query.queryText !== undefined ?
      `&pageState=("traceFilter":("chips":"[${createURIFilterString(query.queryText)}]","traceIntervalPicker":("groupValue":"P1M","customValue":null)))`
      : '';
    const traceParam = query.traceId !== undefined ?
      `&tid=${query.traceId}`
      : '';

    return `https://console.cloud.google.com/traces/list?` +
      timeRangeParam +
      projectParam +
      filterParam +
      traceParam;
  }, [query, range]);

  /**
   * Create a special string for the filter part of the Google Cloud Trace URI
   */
  function createURIFilterString(queryText: string) {
    // Split query string into multiple strings for each part of the filter
    let queryFilters = queryText.match(/(?:[^\s"]+|"(?:\\"|[^"])*")+/g)
    // From each filter part, create Google Cloud Trace URI string portion to match it
    let uriFilterMaps = queryFilters?.map(filterItem => {
      let key = filterItem.substring(0, filterItem.indexOf(":"));
      let value = filterItem.substring(filterItem.indexOf(":") + 1, filterItem.length);

      if (key.toLowerCase() === "label") {
        key = `${key}:${value.substring(0, value.indexOf(":"))}`
        value = value.substring(value.indexOf(":") + 1, value.length);
      }

      let specialChars = ""
      // Attempt to grab any special chars (+ or ^) so we can tack them on after removing quotes
      if (value.length > 1) {
        let firstChar = value.charAt(0)
        let secondChar = value.charAt(1)

        // Move specials chars from the front of value to key for Google Cloud Trace compatibility
        if ((firstChar === "^" && secondChar === "+") || (firstChar === "+" && secondChar === "^")) {
          specialChars = "^+"
          value = value.substring(2, value.length)
        } else if (firstChar === "+" || firstChar === "^") {
          specialChars = firstChar
          value = value.substring(1, value.length)
        }
      }

      // Remove any quotes from value as these cause issues with the URI
      value = value.replace(/(^"|"$)/g, '')
      // Re-add any special characters if any
      value = specialChars + value
      // Convert escaped quotes in value to underscore Hex values for URI compatibility
      value = value.replace(/\\"/gi, "_5C_5C_5C_22")
      // Convert + in value to underscore Hex values for URI compatibility
      value = value.replace("+", "%2B")

      // Return the complete URI portion for this part of the filter
      return `{_22k_22_3A_22${key}_22_2C_22t_22_3A10_2C_22v_22_3A_22_5C_22${value}_5C_22_22}`
    })

    return uriFilterMaps?.join(",")
  }

  const renderExploreBody = () => {
    switch (query.queryType) {
      case 'traceID':
        return (
          <InlineFieldRow>
            <InlineField>
              <Input
                name="TraceID"
                width={50}
                value={query.traceId}
                placeholder={'Enter a Cloud Trace ID (Run with Shift+Enter)'}
                onBlur={onRunQuery}
                onChange={e => onChange({
                  ...query,
                  traceId: e.currentTarget.value,
                  projectId: query.projectId,
                  refId: query.refId,
                })}
                onKeyDown={onKeyDownInput}
              />
            </InlineField>
          </InlineFieldRow>
        );
      default:
        return (
          <TextArea
            name="Query"
            className="slate-query-field"
            value={query.queryText}
            rows={10}
            placeholder="Enter a Cloud Trace query (Run with Shift+Enter)"
            onBlur={onRunQuery}
            onChange={e => onChange({
              ...query,
              queryText: e.currentTarget.value,
              projectId: query.projectId,
              refId: query.refId,
            })}
            onKeyDown={onKeyDownTextArea}
          />
        );
    }
  };

  return (
    <>
      <InlineFieldRow>
        <InlineField label='Project ID'>
          <AsyncSelect
            width={30}
            allowCustomValue
            formatCreateLabel={(v) => `Use project: ${v}`}
            onChange={e => onChange({
              ...query,
              queryText: query.queryText,
              projectId: e.value!,
              refId: query.refId,
            })}
            loadOptions={loadProjects}
            defaultOptions
            value={query.projectId ? { label: query.projectId, value: query.projectId } : undefined}
            placeholder="Select Project"
            inputId={`${query.refId}-project`}
          />
        </InlineField>
      </InlineFieldRow>
      <InlineFieldRow>
        <InlineField label="Query type">
          <RadioButtonGroup<string>
            options={[
              { value: undefined, label: "Filter" },
              { value: 'traceID', label: 'Trace ID' },
            ]}
            value={query.queryType}
            onChange={(v) =>
              onChange({
                ...query,
                queryType: v,
              })
            }
            size="md"
          />
        </InlineField>
      </InlineFieldRow>
      {fetchError && (
        <Alert severity="error" title={fetchError} />
      )}
      {renderExploreBody()}
      <Tooltip content='Click to view these results in the Google Cloud console'>
        <LinkButton
          href={gcpConsoleURI}
          disabled={!gcpConsoleURI}
          target='_blank'
          icon='external-link-alt'
          variant='secondary'
        >
          View in Cloud Trace
        </LinkButton>
      </Tooltip>
    </>
  );
};
