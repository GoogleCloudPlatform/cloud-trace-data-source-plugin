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

import { DataFrame, DataQueryRequest, DataQueryResponse, DataSourceInstanceSettings } from '@grafana/data';
import { BackendSrv, DataSourceWithBackend, getBackendSrv } from '@grafana/runtime';
import { Observable } from 'rxjs';
import { map } from 'rxjs/operators';
import { CloudTraceOptions, Query } from './types';

export class DataSource extends DataSourceWithBackend<Query, CloudTraceOptions> {
  private instanceSettings: DataSourceInstanceSettings<CloudTraceOptions>;
  
  constructor(instanceSettings: DataSourceInstanceSettings<CloudTraceOptions>) {
    super(instanceSettings);

    this.instanceSettings = instanceSettings;
  }

  /**
   * Get the Project ID we parsed from the data source's JWT token
   *
   * @returns Project ID from the provided JWT token
   */
  getDefaultProject(): string {
    return this.instanceSettings.jsonData.defaultProject ?? '';
  }

  /**
   * Have the backend call `resourcemanager.projects.list` with our credentials,
   * and return the IDs of all projects found
   *
   * @param backendSrv  {@link BackendSrv} to make the request, only exposed for tests
   * @returns List of discovered project IDs
   */
  async getProjects(backendSrv: BackendSrv = getBackendSrv()): Promise<string[]> {
    try {
      const res = await backendSrv.get(`/api/datasources/${this.id}/resources/projects`);
      return res.projects;
    } catch (ex: unknown) {
      return [];
    }
  }

  /**
   * Check's the Cloud Trace data query's hide property,
   * and returns whether or not this query should be hidden
   *
   * @param query  {@link Query} to check if hide is currently set
   * @returns Boolean of whether or not to hide the attempted query
   */
  filterQuery(query: Query): boolean {
    return !query.hide;
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
    idField!.config.links = [
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
