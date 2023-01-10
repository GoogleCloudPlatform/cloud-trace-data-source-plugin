import { DataSourcePlugin } from '@grafana/data';
import { DataSource } from './datasource';
import { ConfigEditor } from './ConfigEditor';
import { CloudTraceQueryEditor } from './QueryEditor';
import { CloudTraceOptions, Query } from './types';

export const plugin = new DataSourcePlugin<DataSource, Query, CloudTraceOptions>(DataSource)
  .setConfigEditor(ConfigEditor)
  .setQueryEditor(CloudTraceQueryEditor);
