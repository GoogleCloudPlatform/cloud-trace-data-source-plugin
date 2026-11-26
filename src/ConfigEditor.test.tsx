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

import React from 'react';
import { fireEvent, render, screen } from '@testing-library/react';
import { DataSourceSettings } from '@grafana/data';
import { GoogleAuthType } from '@grafana/google-sdk';
import { ConfigEditor } from './ConfigEditor';
import { CloudTraceOptions, DataSourceSecureJsonData } from './types';

jest.mock('@grafana/runtime', () => ({
  ...jest.requireActual('@grafana/runtime'),
  DataSourcePicker: () => <div data-testid="datasource-picker" />,
}));

function deepFreeze<T>(obj: T): T {
  if (obj && typeof obj === 'object') {
    Object.freeze(obj);
    for (const value of Object.values(obj as Record<string, unknown>)) {
      deepFreeze(value);
    }
  }
  return obj;
}

function makeOptions(
  jsonData: Partial<CloudTraceOptions> = {}
): DataSourceSettings<CloudTraceOptions, DataSourceSecureJsonData> {
  return deepFreeze({
    id: 1,
    uid: 'trace-ds',
    orgId: 1,
    name: 'Cloud Trace',
    type: 'googlecloud-trace-datasource',
    typeLogoUrl: '',
    typeName: 'Google Cloud Trace',
    access: 'proxy',
    url: '',
    user: '',
    database: '',
    basicAuth: false,
    basicAuthUser: '',
    isDefault: false,
    jsonData: {
      authenticationType: GoogleAuthType.JWT,
      ...jsonData,
    } as CloudTraceOptions,
    secureJsonData: {},
    secureJsonFields: {},
    readOnly: false,
    withCredentials: false,
  });
}

describe('ConfigEditor immutability (#44)', () => {
  it.each([
    ['jwt', { authenticationType: GoogleAuthType.JWT }],
    ['gce', { authenticationType: GoogleAuthType.GCE }],
    ['accessToken', { authenticationType: 'accessToken' }],
    ['oauthPassthrough', { authenticationType: 'oauthPassthrough', oauthPassThru: true }],
    ['unset authenticationType', { authenticationType: undefined }],
  ])('renders without mutating frozen props (%s)', (_label, jsonData) => {
    const options = makeOptions(jsonData);
    const onOptionsChange = jest.fn();

    expect(() => render(<ConfigEditor options={options} onOptionsChange={onOptionsChange} />)).not.toThrow();
  });

  it('toggles service account impersonation via onOptionsChange without mutating frozen props', () => {
    const options = makeOptions({ usingImpersonation: false });
    const onOptionsChange = jest.fn();

    render(<ConfigEditor options={options} onOptionsChange={onOptionsChange} />);

    const checkbox = screen.getByLabelText('To impersonate an existing Google Cloud service account.');
    fireEvent.click(checkbox);

    expect(onOptionsChange).toHaveBeenCalledWith(
      expect.objectContaining({
        jsonData: expect.objectContaining({
          usingImpersonation: true,
        }),
      })
    );
    expect(options.jsonData.usingImpersonation).toBe(false);
  });

  it('updates serviceAccountToImpersonate via onOptionsChange without mutating frozen props', () => {
    const options = makeOptions({ usingImpersonation: true, serviceAccountToImpersonate: '' });
    const onOptionsChange = jest.fn();

    render(<ConfigEditor options={options} onOptionsChange={onOptionsChange} />);

    const input = screen.getByLabelText('Service Account');
    fireEvent.change(input, { target: { value: 'impersonated@project.iam.gserviceaccount.com' } });

    expect(onOptionsChange).toHaveBeenCalledWith(
      expect.objectContaining({
        jsonData: expect.objectContaining({
          serviceAccountToImpersonate: 'impersonated@project.iam.gserviceaccount.com',
        }),
      })
    );
    expect(options.jsonData.serviceAccountToImpersonate).toBe('');
  });

  it('reflects external prop updates for impersonation fields without stale local state', () => {
    const onOptionsChange = jest.fn();
    const { rerender } = render(
      <ConfigEditor
        options={makeOptions({
          usingImpersonation: true,
          serviceAccountToImpersonate: 'old@proj.iam.gserviceaccount.com',
        })}
        onOptionsChange={onOptionsChange}
      />
    );

    expect(screen.getByLabelText('Service Account')).toHaveValue('old@proj.iam.gserviceaccount.com');

    rerender(
      <ConfigEditor
        options={makeOptions({
          usingImpersonation: true,
          serviceAccountToImpersonate: 'new@proj.iam.gserviceaccount.com',
        })}
        onOptionsChange={onOptionsChange}
      />
    );

    expect(screen.getByLabelText('Service Account')).toHaveValue('new@proj.iam.gserviceaccount.com');

    rerender(
      <ConfigEditor
        options={makeOptions({
          usingImpersonation: false,
          serviceAccountToImpersonate: 'new@proj.iam.gserviceaccount.com',
        })}
        onOptionsChange={onOptionsChange}
      />
    );

    expect(screen.queryByLabelText('Service Account')).toBeNull();
  });

  it('updates tracesToLogsV2 options via onOptionsChange without mutating frozen props', () => {
    const options = makeOptions({
      tracesToLogsV2: { datasourceUid: 'logs-uid', filterByTraceID: false },
    });
    const onOptionsChange = jest.fn();

    render(<ConfigEditor options={options} onOptionsChange={onOptionsChange} />);

    const toggle = screen.getByLabelText(/Filter by trace ID/);
    fireEvent.click(toggle);

    expect(onOptionsChange).toHaveBeenCalledWith(
      expect.objectContaining({
        jsonData: expect.objectContaining({
          tracesToLogsV2: { datasourceUid: 'logs-uid', filterByTraceID: true },
        }),
      })
    );
    expect(options.jsonData.tracesToLogsV2?.filterByTraceID).toBe(false);
  });
});
