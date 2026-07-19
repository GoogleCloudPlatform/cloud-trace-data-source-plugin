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

jest.mock('@grafana/runtime', () => ({
  ...jest.requireActual('@grafana/runtime'),
  DataSourcePicker: (props: { onChange: (ds: { uid: string }) => void }) => (
    <button onClick={() => props.onChange({ uid: 'logs-uid' })}>datasource-picker</button>
  ),
}));

const makeProps = (tracesToLogsV2?: TraceToLogsOptionsV2): Props => {
  const onOptionsChange = jest.fn();
  return {
    options: {
      id: 1,
      uid: 'trace-ds',
      orgId: 1,
      name: 'Google Cloud Trace',
      type: 'googlecloud-trace-datasource',
      typeName: 'Google Cloud Trace',
      typeLogoUrl: '',
      access: 'proxy',
      url: '',
      user: '',
      database: '',
      basicAuth: false,
      basicAuthUser: '',
      isDefault: false,
      jsonData: { tracesToLogsV2 } as Props['options']['jsonData'],
      secureJsonFields: {},
      readOnly: false,
      withCredentials: false,
    },
    onOptionsChange,
  } as unknown as Props;
};

const lastJsonData = (props: Props): { tracesToLogsV2: TraceToLogsOptionsV2 } => {
  const calls = (props.onOptionsChange as jest.Mock).mock.calls;
  return calls[calls.length - 1][0].jsonData;
};

describe('TraceToLogsSettings', () => {
  it('writes datasourceUid when a data source is picked', () => {
    const props = makeProps();
    render(<TraceToLogsSettings {...props} />);
    fireEvent.click(screen.getByText('datasource-picker'));
    expect(lastJsonData(props).tracesToLogsV2).toEqual({ customQuery: false, datasourceUid: 'logs-uid' });
  });

  it('preserves existing settings when toggling filter by trace ID', () => {
    const props = makeProps({ customQuery: false, datasourceUid: 'logs-uid', spanStartTimeShift: '-5m' });
    render(<TraceToLogsSettings {...props} />);
    fireEvent.click(screen.getByLabelText(/Filter by trace ID/));
    expect(lastJsonData(props).tracesToLogsV2).toEqual({
      customQuery: false,
      datasourceUid: 'logs-uid',
      spanStartTimeShift: '-5m',
      filterByTraceID: true,
    });
  });

  it('toggles filter by span ID', () => {
    const props = makeProps({ customQuery: false });
    render(<TraceToLogsSettings {...props} />);
    fireEvent.click(screen.getByLabelText(/Filter by span ID/));
    expect(lastJsonData(props).tracesToLogsV2.filterBySpanID).toBe(true);
  });

  it('shows an error for an invalid time shift and none for valid ones', () => {
    const props = makeProps({ customQuery: false, spanStartTimeShift: 'xyz', spanEndTimeShift: '-30s' });
    render(<TraceToLogsSettings {...props} />);
    expect(screen.getAllByText('Invalid interval (e.g. 5m, -30s, 1h)')).toHaveLength(1);
  });

  it('accepts fractional intervals like 1.5h', () => {
    const props = makeProps({ customQuery: false, spanStartTimeShift: '1.5h' });
    render(<TraceToLogsSettings {...props} />);
    expect(screen.queryByText('Invalid interval (e.g. 5m, -30s, 1h)')).toBeNull();
  });

  it('writes time shift changes to jsonData', () => {
    const props = makeProps({ customQuery: false });
    render(<TraceToLogsSettings {...props} />);
    fireEvent.change(screen.getAllByPlaceholderText('0')[0], { target: { value: '-1h' } });
    expect(lastJsonData(props).tracesToLogsV2.spanStartTimeShift).toBe('-1h');
  });

  it('adds and removes tags', () => {
    const props = makeProps({ customQuery: false });
    const { rerender } = render(<TraceToLogsSettings {...props} />);
    fireEvent.click(screen.getByText('Add tag'));
    expect(lastJsonData(props).tracesToLogsV2.tags).toEqual([{ key: '' }]);

    // Re-render with the tag present and edit its key
    props.options.jsonData.tracesToLogsV2 = { customQuery: false, tags: [{ key: '' }] };
    rerender(<TraceToLogsSettings {...props} />);
    fireEvent.change(screen.getByLabelText('Tag key 1'), { target: { value: 'service.name' } });
    expect(lastJsonData(props).tracesToLogsV2.tags).toEqual([{ key: 'service.name' }]);

    fireEvent.click(screen.getByLabelText('Remove tag 1'));
    expect(lastJsonData(props).tracesToLogsV2.tags).toEqual([]);
  });

  it('reveals the query field only when custom query is enabled and stores the query', () => {
    const props = makeProps({ customQuery: false });
    const { rerender } = render(<TraceToLogsSettings {...props} />);
    expect(screen.queryByLabelText('Query')).toBeNull();

    fireEvent.click(screen.getByLabelText(/Use custom query/));
    expect(lastJsonData(props).tracesToLogsV2.customQuery).toBe(true);

    props.options.jsonData.tracesToLogsV2 = { customQuery: true };
    rerender(<TraceToLogsSettings {...props} />);
    const textarea = screen.getByRole('textbox', { name: /query/i });
    fireEvent.change(textarea, { target: { value: '"${__span.traceId}"' } });
    expect(lastJsonData(props).tracesToLogsV2.query).toBe('"${__span.traceId}"');
  });
});
