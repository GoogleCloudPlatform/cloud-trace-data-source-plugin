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

import { DataSourcePluginOptionsEditorProps, SelectableValue } from '@grafana/data';
import { ConnectionConfig, GoogleAuthType } from '@grafana/google-sdk';
import { Alert, Field, Input, SecretInput, Select, TextArea } from '@grafana/ui';
import React, { PureComponent } from 'react';
import { TraceToLogsSettings } from './TraceToLogsSettings';
import { authTypes, CloudTraceOptions, DataSourceSecureJsonData } from './types';

export type Props = DataSourcePluginOptionsEditorProps<CloudTraceOptions, DataSourceSecureJsonData>;

/**
 * JWT and GCE authentication are configured through @grafana/google-sdk's
 * ConnectionConfig, which already renders the project field ("Project ID" for
 * JWT, "Default project" for GCE) and the service account impersonation
 * controls. The plugin only renders its own project field for the other
 * authentication types.
 */
const usesConnectionConfig = (authenticationType: string) =>
  authenticationType === GoogleAuthType.JWT || authenticationType === GoogleAuthType.GCE;

export class ConfigEditor extends PureComponent<Props> {
  render() {
    const { options, onOptionsChange } = this.props;
    const secureJsonData = options.secureJsonData || {};
    // The backend treats a missing authentication type as JWT; show it that way too.
    const authenticationType = options.jsonData.authenticationType || GoogleAuthType.JWT;

    const handleAuthTypeChange = (e: SelectableValue<string>) => {
      if (e.value === 'oauthPassthrough') {
        onOptionsChange({
          ...options,
          jsonData: {
            ...options.jsonData,
            authenticationType: 'oauthPassthrough',
            oauthPassThru: true,
          },
        });
      } else {
        onOptionsChange({
          ...options,
          jsonData: {
            ...options.jsonData,
            authenticationType: e.value || GoogleAuthType.JWT,
            oauthPassThru: false,
          },
          secureJsonData: {
            ...options.secureJsonData,
            accessToken: '',
          },
          secureJsonFields: {
            ...options.secureJsonFields,
            accessToken: false,
          },
        });
      }
    };

    return (
      <>
        <Field
          label={'Authentication'}
          description={'Choose the type of authentication for Google Cloud Trace services'}
          htmlFor="authentication-type"
        >
          <Select
            width={30}
            value={authTypes.find((opt) => opt.value === authenticationType)}
            options={authTypes}
            onChange={handleAuthTypeChange}
            disabled={false}
          />
        </Field>
        {usesConnectionConfig(authenticationType) ? (
          // ConnectionConfig keeps the JWT/GCE mode in local state. Remount it when the
          // authentication type changes through the Select above so it never goes stale.
          <ConnectionConfig key={authenticationType} {...this.props}></ConnectionConfig>
        ) : null}
        {authenticationType === GoogleAuthType.GCE ? (
          <Alert title="Application Default Credentials or another project" severity="info">
            Outside GCE (for example after <code>gcloud auth application-default login</code>), or to read traces
            from a project other than the VM&apos;s own, set <strong>Default project</strong> above. It takes
            precedence over the GCE metadata server project for queries and health checks.
          </Alert>
        ) : null}
        {authenticationType === 'accessToken' ? (
          <>
            <Field
              label="Access Token"
              description="Alternatively, configure a temporary access token and a project ID. This will override other authentication methods."
            >
              <SecretInput
                autoComplete="new-password"
                value={secureJsonData.accessToken || ''}
                onChange={(e: React.ChangeEvent<HTMLInputElement>) => {
                  onOptionsChange({
                    ...options,
                    secureJsonData: {
                      ...secureJsonData,
                      accessToken: e.target.value,
                    },
                  });
                }}
                isConfigured={!!options.secureJsonFields?.accessToken}
                onReset={() => {
                  onOptionsChange({
                    ...options,
                    secureJsonData: {
                      ...secureJsonData,
                      accessToken: '',
                    },
                    secureJsonFields: {
                      ...options.secureJsonFields,
                      accessToken: false,
                    },
                  });
                }}
              />
            </Field>
          </>
        ) : null}
        {usesConnectionConfig(authenticationType) ? null : defaultProject(this.props)}
        {connectionOptions(this.props)}
        <TraceToLogsSettings options={this.props.options} onOptionsChange={this.props.onOptionsChange} />
      </>
    );
  }
}

const defaultProject = (props: Props) => {
  const { options, onOptionsChange } = props;
  return (
    <Field
      label="Default Project ID"
      description="Project used by default for queries and health checks. Required for Access Token and OAuth Passthrough authentication."
      htmlFor="default-project"
    >
      <Input
        id="default-project"
        autoComplete="off"
        value={options.jsonData.defaultProject || ''}
        onChange={(e: React.ChangeEvent<HTMLInputElement>) => {
          onOptionsChange({
            ...options,
            jsonData: {
              ...options.jsonData,
              defaultProject: e.target.value,
            },
          });
        }}
      />
    </Field>
  );
};

const connectionOptions = (props: Props) => {
  const { options, onOptionsChange } = props;
  return (
    <>
      <Field label="Universe Domain" description="Optional">
        <Input
          autoComplete="off"
          placeholder="googleapis.com (default)"
          value={options.jsonData.universeDomain || ''}
          onChange={(e: React.ChangeEvent<HTMLInputElement>) => {
            onOptionsChange({
              ...options,
              jsonData: {
                ...options.jsonData,
                universeDomain: e.target.value,
              },
            });
          }}
        />
      </Field>
      <Field
        label="Project List Filter"
        description="Enter project IDs or regex patterns, one per line. Only matching projects will appear in the project dropdown. Leave empty to show all projects."
      >
        <TextArea
          value={options.jsonData.projectListFilter || ''}
          placeholder={'my-project-id\nteam-.*\nprod-.*-trace'}
          rows={4}
          onChange={(e: React.ChangeEvent<HTMLTextAreaElement>) => {
            onOptionsChange({
              ...options,
              jsonData: {
                ...options.jsonData,
                projectListFilter: e.target.value,
              },
            });
          }}
        />
      </Field>
    </>
  );
};
