import React, { KeyboardEvent, useEffect, useMemo, useState } from 'react';
import { QueryEditorProps, SelectableValue } from '@grafana/data';
import { InlineField, InlineFieldRow, Input, LinkButton, RadioButtonGroup, Select, TextArea, Tooltip } from '@grafana/ui';
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

  const [projects, setProjects] = useState<Array<SelectableValue<string>>>();
  useEffect(() => {
    datasource.getProjects().then(res => {
      setProjects(res.map(project => ({
        label: project,
        value: project,
      })));
    });
  }, [datasource]);


  // Apply defaults if needed
  if (query.projectId == null) {
    query.projectId = datasource.getDefaultProject();
  }
  if (query.queryText == null) {
    query.queryText = defaultQuery.queryText;
  }

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
      `&pageState=("traceFilter":("chips":"[${createURIFilterString(query.queryText)}]"))`
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
      var key = filterItem.substring(0, filterItem.indexOf(":"));
      var value = filterItem.substring(filterItem.indexOf(":") + 1, filterItem.length);

      if (key.toLowerCase() === "label") {
        key = `${key}:${value.substring(0, value.indexOf(":"))}`
        value = value.substring(value.indexOf(":") + 1, value.length);
      }
      
      var specialChars = ""
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
            width={30}
            allowCustomValue
            formatCreateLabel={(v) => `Use project: ${v}`}
            onChange={e => onChange({
              ...query,
              queryText: query.queryText,
              projectId: e.value!,
              refId: query.refId,
            })}
            options={projects}
            value={query.projectId}
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
