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

import { DataFrame, DataQueryRequest, DataQueryResponse, DataSourceInstanceSettings, ScopedVars } from '@grafana/data';
import { DataSourceWithBackend, getBackendSrv, getTemplateSrv, TemplateSrv } from '@grafana/runtime';
import { map } from 'rxjs/operators';
import { lastValueFrom, Observable } from 'rxjs';
import { CloudTraceOptions, Query } from './types';
import { CloudTraceVariableSupport } from './variables';

/** How long to wait before retrying a failed GCE default project lookup. */
export const GCE_DEFAULT_PROJECT_RETRY_MS = 60_000;

export class DataSource extends DataSourceWithBackend<Query, CloudTraceOptions> {
  authenticationType: string;
  annotations = {};
  private filterRegexes: RegExp[] | null = null;
  private gceDefaultProject?: string;
  private gceDefaultProjectPromise: Promise<void> | null = null;
  private gceDefaultProjectFailedAt?: number;

  constructor(
    private instanceSettings: DataSourceInstanceSettings<CloudTraceOptions>,
    private readonly templateSrv: TemplateSrv = getTemplateSrv(),
  ) {
    super(instanceSettings);
    this.authenticationType = instanceSettings.jsonData.authenticationType || 'jwt';
    this.variables = new CloudTraceVariableSupport(this);
  }

  /**
   * Override testDatasource to sanitize errors that may contain raw HTML.
   * When Grafana returns HTTP 502 for health checks, the GCP Load Balancer
   * intercepts it and replaces the body with its own HTML 502 page. This
   * override catches those errors and returns a readable message instead.
   */
  async testDatasource() {
    try {
      return await super.testDatasource();
    } catch (err: unknown) {
      const errObj = err as Record<string, unknown>;
      // Grafana's backendSrv puts the HTTP response body in err.data
      const raw = errObj?.['data'] ?? errObj?.['message'] ?? String(err);
      const text = typeof raw === 'string' ? raw : JSON.stringify(raw);
      const isHtml = /<html[\s>]|<!doctype\s+html/i.test(text) || text.includes('text/html');
      return {
        status: 'error' as const,
        message: isHtml
          ? 'The server returned an HTML error page. If you set a Universe Domain, please verify it is correct.'
          : text,
      };
    }
  }

  /**
   * Override getResource to suppress Grafana's automatic error toast popups.
   * The plugin's QueryEditor already shows errors inline via its own Alert,
   * so the default toasts cause every error to appear twice.
   */
  async getResource(path: string, params?: Record<string, unknown>): Promise<any> {
    return lastValueFrom(
      getBackendSrv().fetch({
        url: `/api/datasources/uid/${this.uid}/resources/${path}`,
        params,
        method: 'GET',
        showErrorAlert: false,
      })
    ).then((response) => response.data);
  }

  /**
   * Get the default Project ID from configuration (JWT, OAuth passthrough,
   * or an explicit override for GCE / local Application Default Credentials),
   * falling back to the GCE metadata server when using GCE auth without an
   * explicit defaultProject.
   *
   * @returns Default GCP Project ID, or empty string if unset/unavailable
   */
  async getDefaultProject(): Promise<string> {
    const { defaultProject, authenticationType } = this.instanceSettings.jsonData;
    if (defaultProject) {
      return defaultProject;
    }
    if (authenticationType === 'gce') {
      await this.ensureGCEDefaultProject();
      return this.gceDefaultProject || this.instanceSettings.jsonData.gceDefaultProject || '';
    }

    return '';
  }

  async getGCEDefaultProject() {
    return this.getResource(`gceDefaultProject`);
  }

  /**
   * Resolve and cache the GCE default project in memory without mutating
   * `instanceSettings.jsonData`. Single-flight: concurrent calls share one
   * resource request, and a successful result (including an empty project, as
   * returned for local user ADC) is kept for the lifetime of this instance.
   * When the lookup fails (the backend answers HTTP 502, for example when it
   * finds no credentials), it is not retried for GCE_DEFAULT_PROJECT_RETRY_MS.
   * Variable queries call this on every refresh, so this avoids re-sending a
   * failing request each time without keeping a transient failure until the
   * page is reloaded.
   */
  async ensureGCEDefaultProject(): Promise<void> {
    const { authenticationType, defaultProject, gceDefaultProject } = this.instanceSettings.jsonData;
    if (authenticationType !== 'gce' || defaultProject || gceDefaultProject || this.gceDefaultProject !== undefined) {
      return;
    }
    if (
      this.gceDefaultProjectFailedAt !== undefined &&
      Date.now() - this.gceDefaultProjectFailedAt < GCE_DEFAULT_PROJECT_RETRY_MS
    ) {
      return;
    }
    if (!this.gceDefaultProjectPromise) {
      this.gceDefaultProjectPromise = this.getGCEDefaultProject()
        .then((project) => {
          this.gceDefaultProject = project || '';
          this.gceDefaultProjectFailedAt = undefined;
        })
        .catch(() => {
          this.gceDefaultProjectFailedAt = Date.now();
        })
        .finally(() => {
          this.gceDefaultProjectPromise = null;
        });
    }
    await this.gceDefaultProjectPromise;
  }

  /**
   * Have the backend call `resourcemanager.projects.list` with our credentials,
   * and return the IDs of all projects found.
   * When query is provided, results are filtered server-side.
   * The default (empty) call is cached so multiple query editors don't
   * trigger redundant backend requests.
   *
   * @returns List of discovered project IDs
   */
  private defaultProjectsCache: Promise<string[]> | null = null;

  getProjects(query?: string): Promise<string[]> {
    if (!query) {
      if (!this.defaultProjectsCache) {
        this.defaultProjectsCache = this.getResource('projects').catch((err: unknown) => {
          this.defaultProjectsCache = null;
          throw err;
        });
      }
      return this.defaultProjectsCache;
    }
    return this.getResource('projects', { query });
  }

  /**
   * Filter a list of project IDs against the configured project list filter patterns.
   * Each non-empty line in `projectListFilter` is treated as a regex pattern
   * anchored to the full project ID (^pattern$).
   * If no patterns are configured, all projects pass through unchanged.
   */
  filterProjects(projects: string[]): string[] {
    if (this.filterRegexes === null) {
      const raw = this.instanceSettings.jsonData.projectListFilter;
      if (!raw || !raw.trim()) {
        this.filterRegexes = [];
      } else {
        this.filterRegexes = raw
          .split('\n')
          .map((line: string) => line.trim())
          .filter((line: string) => line.length > 0)
          .map((p: string) => {
            try {
              return new RegExp(`^(?:${p})$`);
            } catch {
              // If invalid regex, escape special chars and treat as literal match
              return new RegExp(`^(?:${p.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')})$`);
            }
          });
      }
    }

    if (this.filterRegexes.length === 0) {
      return projects;
    }
    const regexes = this.filterRegexes;
    return projects.filter((proj: string) => regexes.some((r: RegExp) => r.test(proj)));
  }

  /**
   * Get projects from the API and apply the configured project list filter.
   */
  async getFilteredProjects(query?: string): Promise<string[]> {
    const projects = await this.getProjects(query);
    return this.filterProjects(projects);
  }

  applyTemplateVariables(query: Query, scopedVars: ScopedVars): Query {
    let normalizedQuery = { ...query };

    // Handle Grafana's standard "Query with traces" format
    if (query.queryType === 'traceql' && (query as any).query) {
      normalizedQuery.queryType = 'traceID';
      normalizedQuery.traceId = (query as any).query;
    }

    return {
      ...normalizedQuery,
      queryText: this.templateSrv.replace(normalizedQuery.queryText, scopedVars),
      projectId: this.templateSrv.replace(normalizedQuery.projectId, scopedVars),
      traceId: this.templateSrv.replace(normalizedQuery.traceId || '', scopedVars),
    };
  }

  /**
   * Check's the Cloud Trace data query's hide property,
   * and returns whether or not this query should be hidden
   *
   * @param query  {@link Query} to check if hide is currently set
   * @returns Boolean of whether or not to hide the attempted query
   */
  filterQuery(query: Query): boolean {
    if (query.hide) {
      return false;
    }
    if (query.queryType === 'traceID' && !(query.traceId || '').trim()) {
      return false;
    }
    if (!query.queryType && !query.projectId) {
      return false;
    }
    return true;
  }

  /**
   * After performing a query, performs post-processing on the result
   *
   * @param request  {@link DataQueryRequest<Query>} a data query request
   * @returns a modified {@link Obserservable<DataQueryResponse>}
   */
  query(request: DataQueryRequest<Query>): Observable<DataQueryResponse> {
    let response = super.query(request);
    return response.pipe(
      map((dataQueryResponse) => {
        return {
          ...dataQueryResponse,
          data: dataQueryResponse.data.flatMap((frame) => {
            const query = request.targets.find((t) => t.refId === frame.refId);
            return this.addLinksToTraceIdColumn(frame, query);
          }),
        };
      })
    );
  }

  /**
   * Provides Grafana with the correct query shape for trace ID lookups.
   * This is called when the user clicks "Query with traces" from exemplars,
   * ensuring the query is constructed with the correct queryType and traceId
   * fields that this datasource expects.
   */
  getTraceQuery(traceId: string): Partial<Query> {
    return {
      queryType: 'traceID',
      traceId: traceId,
      projectId: '',
    };
  }

  /**
   * Takes a response data frame, and adds links to the `Trace ID` field
   * of it as long as it is a "traceTable" data frame. These links will perform
   * a new traceID queryType query for this same datasource using the trace ID
   * found in a given field
   *
   * @param request  {@link DataQueryRequest<Query>} a data query request
   * @returns a modified {@link Obserservable<DataQueryResponse>}
   */
  addLinksToTraceIdColumn(response: DataFrame, query?: Query): DataFrame[] {
    if (response.name !== "traceTable") {
      return [response];
    }

    const idField = response.fields.find((f) => f.name === 'Trace ID');
    if (!idField) {
      return [response];
    }
    idField.config = idField.config || {};
    idField.config.links = [
      {
        title: 'Trace: ${__value.raw}',
        url: '',
        internal: {
          datasourceUid: this.instanceSettings.uid,
          datasourceName: this.instanceSettings.name,
          query: {
            ...(query || {}),
            traceId: '${__value.raw}',
            queryType: 'traceID',
          },
        },
      },
    ];
    return [response];
  }
}
