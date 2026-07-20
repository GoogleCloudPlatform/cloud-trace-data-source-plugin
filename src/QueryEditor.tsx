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
import { Alert, InlineField, InlineFieldRow, Input, LinkButton, RadioButtonGroup, Select, TextArea, Tooltip } from '@grafana/ui';
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

  // Keep a ref to the latest query so async callbacks avoid stale closures
  const queryRef = useRef(query);
  queryRef.current = query;

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

  // Initialization effect: a non-empty projectId is kept as-is — it may come
  // from a deliberate source the project list can't vouch for, such as a
  // logs-to-traces link from the Cloud Logging plugin pointing at a project
  // these credentials can read traces in but cannot list (or one excluded by
  // the project filter). A genuinely wrong project fails loudly with an API
  // error naming it, which beats silently querying a different project and
  // reporting "trace not found". Only an empty projectId is reseated to the
  // default (or first available) project, with an auto-rerun.
  useEffect(() => {
    let cancelled = false;

    (async () => {
      const latestQuery = queryRef.current;
      const needsQueryText = latestQuery.queryText == null && defaultQuery.queryText;

      if (latestQuery.projectId) {
        if (!cancelled && needsQueryText) {
          onChange({ ...latestQuery, queryText: defaultQuery.queryText });
        }
        return;
      }

      const defaultProject = await datasource.getDefaultProject();
      if (cancelled) { return; }

      // Pick a project for the empty query.
      let newProjectId = '';
      if (defaultProject && datasource.filterProjects([defaultProject]).length > 0) {
        newProjectId = defaultProject;
      } else {
        try {
          const list = await datasource.getFilteredProjects();
          if (cancelled) { return; }
          if (list.length > 0) {
            newProjectId = list[0];
          }
        } catch {
          // Transient API error — leave the project empty; the picker still
          // lets the user choose one manually.
        }
      }

      if (cancelled) { return; }
      const nextQuery: Query = { ...latestQuery, projectId: newProjectId };
      if (needsQueryText) {
        nextQuery.queryText = defaultQuery.queryText;
      }
      onChange(nextQuery);
      if (newProjectId) {
        onRunQuery();
      }
    })();

    return () => { cancelled = true; };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [datasource]);

  // Project list state, tagged with the DS uid it was loaded for. The uid tag
  // lets the picker's `options` derivation (`projectsForCurrentDs` below)
  // return [] whenever the loaded data doesn't belong to the current DS —
  // preventing react-select from auto-selecting/displaying a stale option
  // from the previous datasource while the new DS's load is in flight.
  const [projectsState, setProjectsState] = useState<{
    uid: string | null;
    list: Array<SelectableValue<string>>;
  }>({ uid: null, list: [] });
  const [projectsLoading, setProjectsLoading] = useState(false);
  const searchTimer = useRef<ReturnType<typeof setTimeout>>();
  // Mutable mirror of the current datasource uid. Used by stale-response
  // guards: setTimeout callbacks close over the OLD datasource object, so
  // comparing `searchDsUid` against `datasource.uid` from the same closure
  // would always be equal. The ref is mutated on every render, so it always
  // reflects the latest uid regardless of which closure is reading it.
  const currentDsUidRef = useRef(datasource.uid);
  currentDsUidRef.current = datasource.uid;
  const projectsForCurrentDs = projectsState.uid === datasource.uid
    ? projectsState.list
    : [];

  useEffect(() => {
    let cancelled = false;
    const loadingForUid = datasource.uid;
    setProjectsLoading(true);
    datasource.getFilteredProjects()
      .then(res => {
        if (cancelled) { return; }
        setProjectsState({
          uid: loadingForUid,
          list: res.map(project => ({ label: project, value: project })),
        });
        setFetchError(undefined);
      })
      .catch(err => {
        if (cancelled) { return; }
        setProjectsState({ uid: loadingForUid, list: [] });
        setFetchError(sanitizeFetchError(err));
      })
      .finally(() => {
        if (!cancelled) { setProjectsLoading(false); }
      });
    return () => {
      cancelled = true;
      if (searchTimer.current) { clearTimeout(searchTimer.current); }
    };
  }, [datasource]);

  const onProjectSearchChange = useCallback((value: string) => {
    if (searchTimer.current) { clearTimeout(searchTimer.current); }
    const searchDsUid = datasource.uid;
    setProjectsLoading(true);
    searchTimer.current = setTimeout(() => {
      datasource.getFilteredProjects(value || undefined)
        .then(res => {
          // Discard the response if the datasource changed during the debounce
          // or in-flight fetch. Compare against the ref (mutated each render)
          // rather than `datasource.uid`, which is captured in the same stale
          // closure as `searchDsUid` and would always compare equal.
          if (searchDsUid !== currentDsUidRef.current) { return; }
          setProjectsState({
            uid: searchDsUid,
            list: res.map(project => ({ label: project, value: project })),
          });
          setFetchError(undefined);
        })
        .catch(err => {
          if (searchDsUid !== currentDsUidRef.current) { return; }
          setFetchError(sanitizeFetchError(err));
        })
        .finally(() => {
          if (searchDsUid === currentDsUidRef.current) { setProjectsLoading(false); }
        });
    }, 300);
  }, [datasource]);

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
          <Select
            key={datasource.uid}
            width={30}
            allowCustomValue
            formatCreateLabel={(v) => `Use project: ${v}`}
            onChange={e => onChange({
              ...query,
              queryText: query.queryText,
              projectId: e.value!,
              refId: query.refId,
            })}
            options={projectsForCurrentDs}
            isLoading={projectsLoading}
            onInputChange={onProjectSearchChange}
            filterOption={() => true}
            // Pass value as a self-contained literal — do NOT derive it by
            // looking up query.projectId in the options list. react-select's
            // internal `cleanValue(value, options)` reconciliation lags under
            // React 18 concurrent rendering, which would make the picker
            // visually stuck on stale data on DS switch (only "fixed" by
            // opening DevTools, which forces a paint flush).
            // The init effect keeps query.projectId valid; the picker just
            // displays whatever it says. See react-select#4936.
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
