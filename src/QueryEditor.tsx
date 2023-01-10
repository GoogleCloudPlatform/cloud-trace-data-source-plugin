import React, { KeyboardEvent, useEffect, useState } from 'react';
import { QueryEditorProps, SelectableValue } from '@grafana/data';
import { InlineField, InlineFieldRow, Select, TextArea } from '@grafana/ui';
import { DataSource } from './datasource';
import { DataSourceOptionsExt, defaultQuery, Query } from './types';

type Props = QueryEditorProps<DataSource, Query, DataSourceOptionsExt>;

/**
 * This is basically copied from {MQLQueryEditor} from the cloud-monitoring data source
 *
 */
export function CloudTraceQueryEditor({ datasource, query, range, onChange, onRunQuery }: React.PropsWithChildren<Props>) {
  const onKeyDown = (event: KeyboardEvent<HTMLTextAreaElement>) => {
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

  return (
    <>
      <InlineFieldRow>
        <InlineField label='Project ID'>
          <Select
            width={30}
            allowCustomValue
            formatCreateLabel={(v) => `Use project: ${v}`}
            onChange={e => onChange({
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
      <TextArea
        name="Query"
        className="slate-query-field"
        value={query.queryText}
        rows={10}
        placeholder="Enter a Cloud Trace query (Run with Shift+Enter)"
        onBlur={onRunQuery}
        onChange={e => onChange({
          queryText: e.currentTarget.value,
          projectId: query.projectId,
          refId: query.refId,
        })}
        onKeyDown={onKeyDown}
      />
    </>
  );
};
