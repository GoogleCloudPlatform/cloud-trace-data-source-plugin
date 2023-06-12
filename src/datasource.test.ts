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

import { ConstantVector, DataSourcePluginMeta, FieldType } from '@grafana/data';
import { GoogleAuthType } from '@grafana/google-sdk';
import { random } from 'lodash';
import { DataSource } from './datasource';


describe('Google Cloud Trace Data Source', () => {
    describe('getDefaultProject', () => {
        it('returns empty string if not set', () => {
            const ds = makeDataSource();
            ds.getDefaultProject().then(r => expect(r).toBe(''));
        });
        it('returns defaultProject from jsonData', () => {
            const projectId = `my-gcp-project-${random(100)}`;
            const ds = new DataSource({
                id: random(100),
                type: 'googlecloud-trace-datasource',
                access: 'direct',
                meta: {} as DataSourcePluginMeta,
                uid: `${random(100)}`,
                jsonData: {
                    authenticationType: GoogleAuthType.JWT,
                    defaultProject: projectId,
                },
                name: 'something',
                readOnly: true,
            });
            ds.getDefaultProject().then(r => expect(r).toBe(projectId));
        });
    });

    describe('filterQuery', () => {
        it('returns true if hide is not set', () => {
            const ds = makeDataSource();
            const query = {
                refId: '1',
                projectId: '1',
            };
            expect(ds.filterQuery(query)).toBe(true);
        });
        it('returns true if hide is set to false', () => {
            const ds = makeDataSource();
            const query = {
                refId: '1',
                projectId: '1',
                hide: false
            };
            expect(ds.filterQuery(query)).toBe(true);
        });
        it('returns false if hide is set to true', () => {
            const ds = makeDataSource();
            const query = {
                refId: '1',
                projectId: '1',
                hide: true
            };
            expect(ds.filterQuery(query)).toBe(false);
        });
    });

    describe('addLinksToTraceIdColumn', () => {
        it('makes no changes when data frame is not named "traceTable"', () => {
            const ds = makeDataSource();
            const frame = makeFrame(ds.uid);
            frame.name = "Wrong Name"
            frame.fields[0].config = {}
            const expectedFrame = makeFrame(ds.uid);
            expectedFrame.name = "Wrong Name"
            expectedFrame.fields[0].config = {}
            const query = {
                refId: '1',
                projectId: '2',
                traceId: '3',
            };
            const result = ds.addLinksToTraceIdColumn(frame, query);
            expect(result.length).toBe(1);
            expect(result[0]).toEqual(expectedFrame);
        });
    });

    describe('addLinksToTraceIdColumn', () => {
        it('adds links when data frame is named "traceTable"', () => {
            const ds = makeDataSource();
            const frame = makeFrame(ds.uid);
            frame.fields[0].config = {}
            const expectedFrame = makeFrame(ds.uid);
            const query = {
                refId: '1',
                projectId: '2',
                traceId: '3',
            };
            const result = ds.addLinksToTraceIdColumn(frame, query);
            expect(result.length).toBe(1);
            expect(result[0]).toEqual(expectedFrame);
        });
    });
});

const makeDataSource = () => {
    return new DataSource({
        id: random(100),
        type: 'googlecloud-trace-datasource',
        access: 'direct',
        meta: {} as DataSourcePluginMeta,
        uid: `${random(100)}`,
        jsonData: {
            authenticationType: GoogleAuthType.JWT,
        },
        name: 'something',
        readOnly: true,
    });
}

const makeFrame = (datasourceUid: string) => {
    const values = new ConstantVector<string>("test", 1)
    const link = {
        title: "Trace: ${__value.raw}",
        url: "",
        internal: {
            datasourceName: "something",
            datasourceUid: datasourceUid,
            query: {
                projectId: "2",
                queryType: "traceID",
                refId: "1",
                traceId: "${__value.raw}",
            },
        },
    };
    const config: any = {
        links: [link]
    }
    const field = {
        name: "Trace ID",
        type: FieldType.string,
        config: config,
        values: values
    };
    const frame = {
        name: "traceTable",
        fields: [field],
        length: 1,
    };

    return frame;
}
