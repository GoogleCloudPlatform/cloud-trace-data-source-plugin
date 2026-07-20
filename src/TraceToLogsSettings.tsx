/**
 * Copyright 2026 Google LLC
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

import { DataSourcePluginOptionsEditorProps } from '@grafana/data';
import { DataSourcePicker } from '@grafana/runtime';
import { Button, Field, FieldSet, IconButton, InlineSwitch, Input, TextArea } from '@grafana/ui';
import React from 'react';
import { CloudTraceOptions, DataSourceSecureJsonData, TraceToLogsOptionsV2, TraceToLogsTag } from './types';

export type Props = DataSourcePluginOptionsEditorProps<CloudTraceOptions, DataSourceSecureJsonData>;

/**
 * Same interval rule Grafana core uses for span time-shift inputs;
 * negative shifts (e.g. -1h) are allowed to widen the range backwards.
 */
const intervalRegex = /^-?\d+(?:\.\d+)?(ms|[Mwdhmsy])$/;
const isInvalidInterval = (value?: string) => !!value && !intervalRegex.test(value);

/**
 * Config section for trace-to-logs correlation. Writes Grafana core's
 * TraceToLogsOptionsV2 shape to jsonData.tracesToLogsV2; Grafana's trace view
 * reads it from there to render "Logs for this span" links.
 */
export function TraceToLogsSettings({ options, onOptionsChange }: Props) {
  const settings: TraceToLogsOptionsV2 = options.jsonData.tracesToLogsV2 ?? {};
  const tags = settings.tags ?? [];

  const update = (patch: Partial<TraceToLogsOptionsV2>) =>
    onOptionsChange({
      ...options,
      jsonData: {
        ...options.jsonData,
        // Retire the legacy V1 key so it never shadows tracesToLogsV2
        tracesToLogs: undefined,
        tracesToLogsV2: { ...settings, ...patch },
      },
    });

  const updateTag = (index: number, patch: Partial<TraceToLogsTag>) => {
    const next = [...tags];
    next[index] = { ...next[index], ...patch };
    update({ tags: next });
  };

  return (
    <FieldSet label="Trace to logs">
      <Field
        label="Data source"
        description="Google Cloud Logging data source the trace is going to navigate to"
        htmlFor="trace-to-logs-datasource"
      >
        <DataSourcePicker
          inputId="trace-to-logs-datasource"
          logs={true}
          filter={(ds) => ds.type === 'googlecloud-logging-datasource'}
          noDefault={true}
          width={40}
          current={settings.datasourceUid ?? null}
          onChange={(ds) => update({ datasourceUid: ds.uid })}
          onClear={() => update({ datasourceUid: undefined })}
        />
      </Field>
      <Field
        label="Span start time shift"
        description="Shifts the start of the logs time range relative to the span start. Use a negative value (e.g. -1h) to search earlier."
        invalid={isInvalidInterval(settings.spanStartTimeShift)}
        error="Invalid interval (e.g. 5m, -30s, 1h)"
      >
        <Input
          id="trace-to-logs-start-shift"
          width={30}
          placeholder="0"
          value={settings.spanStartTimeShift || ''}
          onChange={(e: React.ChangeEvent<HTMLInputElement>) => update({ spanStartTimeShift: e.target.value })}
        />
      </Field>
      <Field
        label="Span end time shift"
        description="Shifts the end of the logs time range relative to the span end."
        invalid={isInvalidInterval(settings.spanEndTimeShift)}
        error="Invalid interval (e.g. 5m, -30s, 1h)"
      >
        <Input
          id="trace-to-logs-end-shift"
          width={30}
          placeholder="0"
          value={settings.spanEndTimeShift || ''}
          onChange={(e: React.ChangeEvent<HTMLInputElement>) => update({ spanEndTimeShift: e.target.value })}
        />
      </Field>
      <Field
        label="Tags"
        description="Span tags to include in the logs query. The optional value renames the tag key in the query."
      >
        <div>
          {tags.map((tag, i) => (
            <div key={i} style={{ display: 'flex', gap: '8px', marginBottom: '8px' }}>
              <Input
                aria-label={`Tag key ${i + 1}`}
                width={20}
                placeholder="key"
                value={tag.key}
                onChange={(e: React.ChangeEvent<HTMLInputElement>) => updateTag(i, { key: e.target.value })}
              />
              <Input
                aria-label={`Tag value ${i + 1}`}
                width={20}
                placeholder="new name (optional)"
                value={tag.value || ''}
                onChange={(e: React.ChangeEvent<HTMLInputElement>) =>
                  updateTag(i, { value: e.target.value || undefined })
                }
              />
              <IconButton
                name="times"
                aria-label={`Remove tag ${i + 1}`}
                onClick={() => update({ tags: tags.filter((_, j) => j !== i) })}
              />
            </div>
          ))}
          <Button
            icon="plus"
            variant="secondary"
            type="button"
            onClick={() => update({ tags: [...tags, { key: '' }] })}
          >
            Add tag
          </Button>
        </div>
      </Field>
      <Field label="Filter by trace ID" description="Adds the trace ID to the logs query">
        <InlineSwitch
          id="trace-to-logs-filter-by-trace-id"
          value={settings.filterByTraceID ?? false}
          onChange={(e: React.FormEvent<HTMLInputElement>) => update({ filterByTraceID: e.currentTarget.checked })}
        />
      </Field>
      <Field label="Filter by span ID" description="Adds the span ID to the logs query">
        <InlineSwitch
          id="trace-to-logs-filter-by-span-id"
          value={settings.filterBySpanID ?? false}
          onChange={(e: React.FormEvent<HTMLInputElement>) => update({ filterBySpanID: e.currentTarget.checked })}
        />
      </Field>
      <Field
        label="Use custom query"
        description={
          'Use a custom Cloud Logging query with variables ${__span.traceId}, ${__span.spanId}, ${__span.tags.X} and $__tags'
        }
      >
        <InlineSwitch
          id="trace-to-logs-custom-query"
          value={settings.customQuery ?? false}
          onChange={(e: React.FormEvent<HTMLInputElement>) => update({ customQuery: e.currentTarget.checked })}
        />
      </Field>
      {settings.customQuery && (
        <Field label="Query" htmlFor="trace-to-logs-query">
          <TextArea
            id="trace-to-logs-query"
            rows={3}
            placeholder={'trace="projects/my-project/traces/${__span.traceId}"'}
            value={settings.query || ''}
            onChange={(e: React.ChangeEvent<HTMLTextAreaElement>) => update({ query: e.target.value })}
          />
        </Field>
      )}
    </FieldSet>
  );
}
