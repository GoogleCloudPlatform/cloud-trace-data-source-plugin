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

import { SelectableValue } from '@grafana/data';
import { DataSource } from './datasource';
import { CloudTraceVariableQuery, TraceVariables } from './types';

export default class CloudTraceVariableFindQuery {
    constructor(private datasource: DataSource) { }

    async execute(query: CloudTraceVariableQuery) {
        try {
            if (!query.projectId) {
                this.datasource.getDefaultProject().then(r => query.projectId = r);
            }
            switch (query.selectedQueryType) {
                case TraceVariables.Projects:
                    return this.handleProjectsQuery();
                default:
                    return [];
            }
        } catch (error) {
            console.error(`Could not run CloudTraceVariableFindQuery ${query}`, error);
            return [];
        }
    }

    async handleProjectsQuery() {
        const projects = await this.datasource.getProjects();
        return (projects).map((s) => ({
            text: s,
            value: s,
            expandable: true,
        } as SelectableValue<string>));
    }
}
