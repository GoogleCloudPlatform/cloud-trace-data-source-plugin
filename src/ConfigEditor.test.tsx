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
  jsonData: Partial<CloudTraceOptions> = {},
  secureJsonFields: Record<string, boolean> = {}
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
    secureJsonFields,
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

    // The impersonation switch is rendered by @grafana/google-sdk's ConnectionConfig.
    const toggle = document.getElementById('usingImpersonation') as HTMLInputElement;
    expect(toggle).not.toBeNull();
    fireEvent.click(toggle);

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

    const input = screen.getByLabelText('Service account to impersonate');
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

    expect(screen.getByLabelText('Service account to impersonate')).toHaveValue('old@proj.iam.gserviceaccount.com');

    rerender(
      <ConfigEditor
        options={makeOptions({
          usingImpersonation: true,
          serviceAccountToImpersonate: 'new@proj.iam.gserviceaccount.com',
        })}
        onOptionsChange={onOptionsChange}
      />
    );

    expect(screen.getByLabelText('Service account to impersonate')).toHaveValue('new@proj.iam.gserviceaccount.com');

    rerender(
      <ConfigEditor
        options={makeOptions({
          usingImpersonation: false,
          serviceAccountToImpersonate: 'new@proj.iam.gserviceaccount.com',
        })}
        onOptionsChange={onOptionsChange}
      />
    );

    expect(screen.queryByLabelText('Service account to impersonate')).toBeNull();
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

const projectInputs = (container: HTMLElement) => container.querySelectorAll('input#defaultProject, input#default-project');

describe('ConfigEditor authentication controls', () => {
  it.each([
    ['jwt', { authenticationType: GoogleAuthType.JWT, usingImpersonation: true }],
    ['gce', { authenticationType: GoogleAuthType.GCE, usingImpersonation: true }],
  ])('renders a single set of impersonation controls (%s)', (_label, jsonData) => {
    render(<ConfigEditor options={makeOptions(jsonData)} onOptionsChange={jest.fn()} />);

    expect(screen.queryByLabelText('To impersonate an existing Google Cloud service account.')).toBeNull();
    expect(document.querySelectorAll('#usingImpersonation')).toHaveLength(1);
    expect(screen.getAllByLabelText('Service account to impersonate')).toHaveLength(1);
  });

  it.each([
    ['accessToken', { authenticationType: 'accessToken' }],
    ['oauthPassthrough', { authenticationType: 'oauthPassthrough', oauthPassThru: true }],
  ])('does not offer impersonation where the backend ignores it (%s)', (_label, jsonData) => {
    render(<ConfigEditor options={makeOptions(jsonData)} onOptionsChange={jest.fn()} />);

    expect(document.getElementById('usingImpersonation')).toBeNull();
    expect(screen.queryByLabelText('To impersonate an existing Google Cloud service account.')).toBeNull();
  });

  it('renders exactly one default project field for GCE', () => {
    const { container } = render(
      <ConfigEditor options={makeOptions({ authenticationType: GoogleAuthType.GCE, defaultProject: 'p' })} onOptionsChange={jest.fn()} />
    );

    const inputs = projectInputs(container);
    expect(inputs).toHaveLength(1);
    expect(inputs[0]).toHaveValue('p');
  });

  it.each([
    ['accessToken', { authenticationType: 'accessToken' }],
    ['oauthPassthrough', { authenticationType: 'oauthPassthrough', oauthPassThru: true }],
  ])('renders the plugin default project field (%s)', (_label, jsonData) => {
    const { container } = render(
      <ConfigEditor options={makeOptions({ ...jsonData, defaultProject: 'p' })} onOptionsChange={jest.fn()} />
    );

    const inputs = projectInputs(container);
    expect(inputs).toHaveLength(1);
    expect(inputs[0]).toHaveAttribute('id', 'default-project');
    expect(inputs[0]).toHaveValue('p');
  });

  it('shows the JWT key form after switching from GCE to JWT', () => {
    const onOptionsChange = jest.fn();
    const { rerender } = render(
      <ConfigEditor options={makeOptions({ authenticationType: GoogleAuthType.GCE })} onOptionsChange={onOptionsChange} />
    );
    expect(screen.queryByText('JWT Key Details')).toBeNull();

    rerender(
      <ConfigEditor options={makeOptions({ authenticationType: GoogleAuthType.JWT })} onOptionsChange={onOptionsChange} />
    );
    expect(screen.getByText('JWT Key Details')).toBeInTheDocument();
  });

  it('hides the JWT key form after switching from JWT to GCE', () => {
    const onOptionsChange = jest.fn();
    const { rerender } = render(
      <ConfigEditor options={makeOptions({ authenticationType: GoogleAuthType.JWT })} onOptionsChange={onOptionsChange} />
    );
    expect(screen.getByText('JWT Key Details')).toBeInTheDocument();

    rerender(
      <ConfigEditor options={makeOptions({ authenticationType: GoogleAuthType.GCE })} onOptionsChange={onOptionsChange} />
    );
    expect(screen.queryByText('JWT Key Details')).toBeNull();
  });

  it('treats a missing authentication type as JWT, like the backend', () => {
    render(<ConfigEditor options={makeOptions({ authenticationType: undefined })} onOptionsChange={jest.fn()} />);

    expect(screen.getByText('JWT Key Details')).toBeInTheDocument();
  });

  it('treats a missing authentication type with a stored token as Access Token, like the backend', () => {
    const { container } = render(
      <ConfigEditor
        options={makeOptions({ authenticationType: undefined, defaultProject: 'p' }, { accessToken: true })}
        onOptionsChange={jest.fn()}
      />
    );

    expect(screen.queryByText('JWT Key Details')).toBeNull();
    expect(screen.getByRole('button', { name: 'Reset' })).toBeInTheDocument();
    const inputs = projectInputs(container);
    expect(inputs).toHaveLength(1);
    expect(inputs[0]).toHaveAttribute('id', 'default-project');
    expect(inputs[0]).toHaveValue('p');
  });

  it('renders no project field for JWT until a key is provided', () => {
    const { container } = render(
      <ConfigEditor options={makeOptions({ authenticationType: GoogleAuthType.JWT })} onOptionsChange={jest.fn()} />
    );

    expect(screen.getByText('JWT Key Details')).toBeInTheDocument();
    expect(projectInputs(container)).toHaveLength(0);
  });

  it('renders exactly one project field for a configured JWT key', () => {
    const { container } = render(
      <ConfigEditor
        options={makeOptions(
          {
            authenticationType: GoogleAuthType.JWT,
            clientEmail: 'reader@p.iam.gserviceaccount.com',
            defaultProject: 'p',
            tokenUri: 'https://oauth2.googleapis.com/token',
          },
          { privateKey: true }
        )}
        onOptionsChange={jest.fn()}
      />
    );

    const inputs = projectInputs(container);
    expect(inputs).toHaveLength(1);
    expect(inputs[0]).toHaveAttribute('id', 'defaultProject');
    expect(inputs[0]).toHaveValue('p');
  });
});
