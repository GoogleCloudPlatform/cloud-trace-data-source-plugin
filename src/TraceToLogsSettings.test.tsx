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

import { fireEvent, render, screen } from '@testing-library/react';
import React from 'react';
import { Props, TraceToLogsSettings } from './TraceToLogsSettings';
import { TraceToLogsOptionsV2 } from './types';

interface PickerMockProps {
  onChange: (ds: { uid: string }) => void;
  filter?: (ds: { type: string }) => boolean;
}

let mockPickerProps: PickerMockProps | undefined;

jest.mock('@grafana/runtime', () => ({
  ...jest.requireActual('@grafana/runtime'),
  DataSourcePicker: (props: PickerMockProps) => {
    mockPickerProps = props;
    return <button onClick={() => props.onChange({ uid: 'logs-uid' })}>datasource-picker</button>;
  },
}));

const makeProps = (tracesToLogsV2?: TraceToLogsOptionsV2): Props =>
  ({ options: { jsonData: { tracesToLogsV2 } }, onOptionsChange: jest.fn() } as unknown as Props);

const lastJsonData = (props: Props): { tracesToLogs?: unknown; tracesToLogsV2: TraceToLogsOptionsV2 } => {
  const calls = (props.onOptionsChange as jest.Mock).mock.calls;
  return calls[calls.length - 1][0].jsonData;
};

const lastSettings = (props: Props): TraceToLogsOptionsV2 => lastJsonData(props).tracesToLogsV2;

describe('TraceToLogsSettings', () => {
  it('seeds trace ID filter and time shifts when a data source is first picked', () => {
    const props = makeProps();
    render(<TraceToLogsSettings {...props} />);
    fireEvent.click(screen.getByText('datasource-picker'));
    expect(lastSettings(props)).toEqual({
      datasourceUid: 'logs-uid',
      filterByTraceID: true,
      spanStartTimeShift: '-5m',
      spanEndTimeShift: '5m',
    });
  });

  it('preserves explicit user choices when re-picking a data source', () => {
    const props = makeProps({
      datasourceUid: 'old-uid',
      filterByTraceID: false,
      spanStartTimeShift: '',
      spanEndTimeShift: '1h',
    });
    render(<TraceToLogsSettings {...props} />);
    fireEvent.click(screen.getByText('datasource-picker'));
    expect(lastSettings(props)).toEqual({
      datasourceUid: 'logs-uid',
      filterByTraceID: false,
      spanStartTimeShift: '',
      spanEndTimeShift: '1h',
    });
  });

  it('only offers Google Cloud Logging data sources', () => {
    render(<TraceToLogsSettings {...makeProps()} />);
    expect(mockPickerProps?.filter?.({ type: 'googlecloud-logging-datasource' })).toBe(true);
    expect(mockPickerProps?.filter?.({ type: 'loki' })).toBe(false);
  });

  it('preserves existing settings when toggling filter by trace ID', () => {
    const props = makeProps({ datasourceUid: 'logs-uid', spanStartTimeShift: '-5m' });
    render(<TraceToLogsSettings {...props} />);
    fireEvent.click(screen.getByLabelText(/Filter by trace ID/));
    expect(lastSettings(props)).toEqual({
      datasourceUid: 'logs-uid',
      spanStartTimeShift: '-5m',
      filterByTraceID: true,
    });
  });

  it('toggles filter by span ID', () => {
    const props = makeProps();
    render(<TraceToLogsSettings {...props} />);
    fireEvent.click(screen.getByLabelText(/Filter by span ID/));
    expect(lastSettings(props).filterBySpanID).toBe(true);
  });

  it('clears the legacy tracesToLogs key on write', () => {
    const props = makeProps();
    (props.options.jsonData as { tracesToLogs?: unknown }).tracesToLogs = { datasourceUid: 'old' };
    render(<TraceToLogsSettings {...props} />);
    fireEvent.click(screen.getByLabelText(/Filter by trace ID/));
    expect(lastJsonData(props).tracesToLogs).toBeUndefined();
  });

  it('shows an error for an invalid time shift and none for valid ones', () => {
    const props = makeProps({ spanStartTimeShift: 'xyz', spanEndTimeShift: '-30s' });
    render(<TraceToLogsSettings {...props} />);
    expect(screen.getAllByText('Invalid interval (e.g. 5m, -30s, 1h)')).toHaveLength(1);
  });

  it('accepts fractional intervals like 1.5h', () => {
    const props = makeProps({ spanStartTimeShift: '1.5h' });
    render(<TraceToLogsSettings {...props} />);
    expect(screen.queryByText('Invalid interval (e.g. 5m, -30s, 1h)')).toBeNull();
  });

  it('writes time shift changes to jsonData', () => {
    const props = makeProps();
    render(<TraceToLogsSettings {...props} />);
    fireEvent.change(screen.getAllByPlaceholderText('0')[0], { target: { value: '-1h' } });
    expect(lastSettings(props).spanStartTimeShift).toBe('-1h');
  });

  it('adds and removes tags', () => {
    const props = makeProps();
    const { rerender } = render(<TraceToLogsSettings {...props} />);
    fireEvent.click(screen.getByText('Add tag'));
    expect(lastSettings(props).tags).toEqual([{ key: '' }]);

    // Re-render with the tag present and edit its key
    props.options.jsonData.tracesToLogsV2 = { tags: [{ key: '' }] };
    rerender(<TraceToLogsSettings {...props} />);
    fireEvent.change(screen.getByLabelText('Tag key 1'), { target: { value: 'service.name' } });
    expect(lastSettings(props).tags).toEqual([{ key: 'service.name' }]);

    fireEvent.click(screen.getByLabelText('Remove tag 1'));
    expect(lastSettings(props).tags).toEqual([]);
  });

  it('defaults the custom query switch to off when provisioned settings omit customQuery', () => {
    // Shape from the README provisioning example: no customQuery key
    const props = makeProps({ datasourceUid: 'logs-uid', spanStartTimeShift: '-5m', filterByTraceID: true });
    const errorSpy = jest.spyOn(console, 'error').mockImplementation(() => {});
    const { rerender } = render(<TraceToLogsSettings {...props} />);
    const switchInput = screen.getByLabelText(/Use custom query/) as HTMLInputElement;
    expect(switchInput.checked).toBe(false);
    expect(screen.queryByLabelText('Query')).toBeNull();

    fireEvent.click(switchInput);
    // Re-render with the toggled value: with value={undefined} the switch would go
    // uncontrolled-to-controlled here and React would warn
    props.options.jsonData.tracesToLogsV2 = { ...props.options.jsonData.tracesToLogsV2, customQuery: true };
    rerender(<TraceToLogsSettings {...props} />);
    expect(errorSpy).not.toHaveBeenCalled();
    errorSpy.mockRestore();
    expect(lastSettings(props)).toEqual({
      datasourceUid: 'logs-uid',
      spanStartTimeShift: '-5m',
      filterByTraceID: true,
      customQuery: true,
    });
  });

  it('reveals the query field only when custom query is enabled and stores the query', () => {
    const props = makeProps();
    const { rerender } = render(<TraceToLogsSettings {...props} />);
    expect(screen.queryByLabelText('Query')).toBeNull();

    fireEvent.click(screen.getByLabelText(/Use custom query/));
    expect(lastSettings(props).customQuery).toBe(true);

    props.options.jsonData.tracesToLogsV2 = { customQuery: true };
    rerender(<TraceToLogsSettings {...props} />);
    const textarea = screen.getByRole('textbox', { name: /query/i });
    fireEvent.change(textarea, { target: { value: '"${__span.traceId}"' } });
    expect(lastSettings(props).query).toBe('"${__span.traceId}"');
  });
});
