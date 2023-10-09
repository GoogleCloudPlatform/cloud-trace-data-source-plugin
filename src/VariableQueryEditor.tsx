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

import React, { PureComponent } from 'react';
import { VariableQueryField } from './Fields';
import { QueryEditorProps } from '@grafana/data';
import { DataSource } from './datasource';
import { CloudTraceVariableQuery, TraceVariables, VariableScopeData, CloudTraceOptions, Query } from './types';

export type Props = QueryEditorProps<DataSource, Query, CloudTraceOptions, CloudTraceVariableQuery>;


export class CloudTraceVariableQueryEditor extends PureComponent<Props, VariableScopeData> {
    queryTypes: Array<{ value: string; label: string }> = [
        { value: TraceVariables.Projects, label: 'Projects' },
    ];

    defaults: VariableScopeData = {
        selectedQueryType: this.queryTypes[0].value,
        projects: [],
        projectId: '',
        loading: true,
    };

    constructor(props: Props) {
        super(props);
        this.state = Object.assign(this.defaults, this.props.query);
    }

    async componentDidMount() {
        await this.props.datasource.ensureGCEDefaultProject();
        const projectId = this.props.query.projectId || (await this.props.datasource.getDefaultProject());
        const projects = (await this.props.datasource.getProjects());
      
        const state: any = {
            projects,
            loading: false,
            projectId,
        };
        this.setState(state, () => this.onPropsChange());
    }

    onPropsChange = () => {
        const { ...queryModel } = this.state;
        this.props.onChange({ ...queryModel, refId: 'CloudTraceVariableQueryEditor-VariableQuery' });
    };

    async onQueryTypeChange(queryType: string) {
        const state: any = {
            selectedQueryType: queryType,
        };

        this.setState(state);
    }

    render() {
        if (this.state.loading) {
            return (
                <div className="gf-form max-width-21">
                    <span className="gf-form-label width-10 query-keyword">Projects</span>
                    <div className="gf-form-select-wrapper max-width-12">
                        <select className="gf-form-input">
                            <option>Loading...</option>
                        </select>
                    </div>
                </div>
            );
        }

        return (
            <>
                <VariableQueryField
                    value={this.state.selectedQueryType}
                    options={this.queryTypes}
                    onChange={(value) => this.onQueryTypeChange(value)}
                    label="Trace Projects"
                />
            </>
        );
    }
}
