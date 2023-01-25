import { DataQuery } from '@grafana/data';
import { DataSourceOptions } from '@grafana/google-sdk';

/**
 * DataSourceOptionsExt adds any extra data to DataSourceOptions
 */
export interface DataSourceOptionsExt extends DataSourceOptions {
}

/**
 * Query from Grafana
 */
export interface Query extends DataQuery {
  queryText?: string;
  projectId: string;
}

/**
 * Query that basically gets all traces
 */
export const defaultQuery: Partial<Query> = {
  queryText: `MinLatency:100ms`,
};

/**
 * These are options configured for each DataSource instance.
 */
export type CloudTraceOptions = DataSourceOptionsExt;
