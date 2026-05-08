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

import { DataSourcePluginMeta, FieldType } from '@grafana/data';
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
        it('returns false for traceID query with empty traceId', () => {
            const ds = makeDataSource();
            const query = {
                refId: '1',
                projectId: '1',
                queryType: 'traceID',
                traceId: '   ',
            };
            expect(ds.filterQuery(query)).toBe(false);
        });
        it('returns true for traceID query with valid traceId', () => {
            const ds = makeDataSource();
            const query = {
                refId: '1',
                projectId: '1',
                queryType: 'traceID',
                traceId: 'abc123',
            };
            expect(ds.filterQuery(query)).toBe(true);
        });
        it('returns false for filter query without projectId', () => {
            const ds = makeDataSource();
            const query = {
                refId: '1',
                projectId: '',
            };
            expect(ds.filterQuery(query)).toBe(false);
        });
        it('returns true for filter query with projectId', () => {
            const ds = makeDataSource();
            const query = {
                refId: '1',
                projectId: 'my-project',
            };
            expect(ds.filterQuery(query)).toBe(true);
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
        it('returns unmodified frame when Trace ID field is missing', () => {
            const ds = makeDataSource();
            const frame = {
                name: 'traceTable',
                fields: [{ name: 'Other Field', type: 'string' as any, config: {}, values: [] as any }],
                length: 0,
            };
            const result = ds.addLinksToTraceIdColumn(frame, undefined);
            expect(result.length).toBe(1);
            expect(result[0]).toEqual(frame);
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

    describe('filterProjects', () => {
        const allProjects = [
            'my-project-123',
            'team-alpha-prod',
            'team-alpha-staging',
            'team-beta-prod',
            'prod-trace-service',
            'other-project',
        ];

        it('returns all projects when no filter is configured', () => {
            const ds = makeDataSource();
            expect(ds.filterProjects(allProjects)).toEqual(allProjects);
        });

        it('returns all projects when filter is empty string', () => {
            const ds = makeDataSource({ projectListFilter: '' });
            expect(ds.filterProjects(allProjects)).toEqual(allProjects);
        });

        it('returns all projects when filter is only whitespace', () => {
            const ds = makeDataSource({ projectListFilter: '   \n  \n  ' });
            expect(ds.filterProjects(allProjects)).toEqual(allProjects);
        });

        it('filters by exact literal project ID', () => {
            const ds = makeDataSource({ projectListFilter: 'my-project-123' });
            expect(ds.filterProjects(allProjects)).toEqual(['my-project-123']);
        });

        it('filters using regex pattern', () => {
            const ds = makeDataSource({ projectListFilter: 'team-alpha-.*' });
            expect(ds.filterProjects(allProjects)).toEqual([
                'team-alpha-prod',
                'team-alpha-staging',
            ]);
        });

        it('supports multiple patterns (union of matches)', () => {
            const ds = makeDataSource({
                projectListFilter: 'my-project-123\nteam-beta-.*',
            });
            expect(ds.filterProjects(allProjects)).toEqual([
                'my-project-123',
                'team-beta-prod',
            ]);
        });

        it('ignores empty lines between patterns', () => {
            const ds = makeDataSource({
                projectListFilter: 'my-project-123\n\n\nother-project',
            });
            expect(ds.filterProjects(allProjects)).toEqual([
                'my-project-123',
                'other-project',
            ]);
        });

        it('anchors patterns so partial matches do not pass', () => {
            const ds = makeDataSource({ projectListFilter: 'team' });
            expect(ds.filterProjects(allProjects)).toEqual([]);
        });

        it('handles invalid regex gracefully by treating as literal', () => {
            const ds = makeDataSource({ projectListFilter: 'invalid[regex' });
            // Should not throw, and should not match anything (literal "invalid[regex" not in list)
            expect(ds.filterProjects(allProjects)).toEqual([]);
        });

        it('trims whitespace from pattern lines', () => {
            const ds = makeDataSource({ projectListFilter: '  my-project-123  ' });
            expect(ds.filterProjects(allProjects)).toEqual(['my-project-123']);
        });

        it('anchors alternations correctly (fixes ^a|b$ bug)', () => {
            const ds = makeDataSource({ projectListFilter: 'team-alpha|team-beta' });
            expect(ds.filterProjects([
                'team-alpha', 
                'team-beta', 
                'team-alpha-prod', 
                'prod-team-beta'
            ])).toEqual([
                'team-alpha', 
                'team-beta'
            ]);
        });

        it('returns empty array when no projects match', () => {
            const ds = makeDataSource({ projectListFilter: 'nonexistent-.*' });
            expect(ds.filterProjects(allProjects)).toEqual([]);
        });
    });

    describe('applyTemplateVariables', () => {
        it('normalizes traceql query from "Query with traces" to traceID format', () => {
            const ds = makeDataSourceWithTemplateSrv();
            const traceId = '52b1a992074849e0beba5eff8c65acf0';
            const query = {
                refId: 'A',
                projectId: 'my-project',
                queryType: 'traceql',
                query: traceId,
                queryText: 'MinLatency:100ms',
            } as any;
            const result = ds.applyTemplateVariables(query, {});
            expect(result.queryType).toBe('traceID');
            expect(result.traceId).toBe(traceId);
        });

        it('leaves standard traceID queries unchanged', () => {
            const ds = makeDataSourceWithTemplateSrv();
            const query = {
                refId: 'A',
                projectId: 'my-project',
                queryType: 'traceID',
                traceId: 'abc123',
            };
            const result = ds.applyTemplateVariables(query, {});
            expect(result.queryType).toBe('traceID');
            expect(result.traceId).toBe('abc123');
        });

        it('leaves filter queries unchanged', () => {
            const ds = makeDataSourceWithTemplateSrv();
            const query = {
                refId: 'A',
                projectId: 'my-project',
                queryText: 'MinLatency:100ms',
            };
            const result = ds.applyTemplateVariables(query, {});
            expect(result.queryType).toBeUndefined();
            expect(result.traceId).toBe('');
        });
    });
});

const makeDataSource = (overrides?: { projectListFilter?: string }) => {
    return new DataSource({
        id: random(100),
        type: 'googlecloud-trace-datasource',
        access: 'direct',
        meta: {} as DataSourcePluginMeta,
        uid: `${random(100)}`,
        jsonData: {
            authenticationType: GoogleAuthType.JWT,
            ...overrides,
        },
        name: 'something',
        readOnly: true,
    });
}

const makeDataSourceWithTemplateSrv = () => {
    const mockTemplateSrv = {
        replace: (s: string) => s,
        getVariables: () => [],
        containsTemplate: () => false,
        updateTimeRange: () => { },
    } as any;
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
    }, mockTemplateSrv);
}

const makeFrame = (datasourceUid: string) => {
    const values = ['test'];
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
