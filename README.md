# Google Cloud Trace Data Source

## Overview
The Google Cloud Trace Data Source is a backend data source plugin for Grafana,
which allows users to query and visualize their Google Cloud traces and spans in Grafana.

![image info](https://github.com/GoogleCloudPlatform/cloud-trace-data-source-plugin/blob/main/src/img/cloud_trace_explore_view.png?raw=true)

## Supported Grafana Version
9.0.x+

## Setup

### Download
Download this plugin to the machine Grafana is running on, either using `git clone` or simply downloading it as a ZIP file. For the purpose of this guide, we'll assume the user "alice" has downloaded it into their local directory "/Users/alice/grafana/". If you are running the Grafana server using a user such as `grafana`, make sure the user has access to the directory.

### Enable Cloud Resource Manager API

You need to enable the resource manager API. Otherwise, your cloud projects will not be displayed in the dropdown menu.

You can follow the steps to enable it:

1. Navigate to the [cloud resource manager API page](https://console.cloud.google.com/apis/library/cloudresourcemanager.googleapis.com) in GCP and select your project
2. Press the `Enable` button

### Authentication

The plugin supports multiple authentication methods. You can select the authentication type from the dropdown in the configuration editor.

#### Google JWT File

1. If you don't have gcp project, add a new gcp project. [link](https://cloud.google.com/resource-manager/docs/creating-managing-projects#console)
2. Open the [Credentials](https://console.developers.google.com/apis/credentials) page in the Google API Console
3. Click **Create Credentials** then click **Service account**
4. On the Create service account page, enter the Service account details
5. On the `Create service account` page, fill in the `Service account details` and then click `Create and Continue`
6. On the `Grant this service account access to project` section, add the `Cloud Trace User` role under `Cloud Trace` to the service account. Click `Done`
7. In the next step, click the service account you just created. Under the `Keys` tab and select `Add key` and `Create new key`
8. Choose key type `JSON` and click `Create`. A JSON key file will be created and downloaded to your computer

If you want to access traces in multiple cloud projects, you need to ensure the service account has permission to read logs from all of them.

#### GCE Default Service Account

If you host Grafana on a GCE VM, you can use the [Compute Engine service account](https://cloud.google.com/compute/docs/access/service-accounts#serviceaccount). You need to make sure the service account has sufficient permissions to access the traces in all projects.

#### Access Token

Similar to [Prometheus data sources on Google Cloud](https://cloud.google.com/stackdriver/docs/managed-prometheus/query#use-serverless), you can configure a scheduled job to use an OAuth2 access token to view the traces. Please follow the steps in the [data source syncer README](https://github.com/GoogleCloudPlatform/cloud-trace-data-source-plugin/blob/main/datasource-syncer/README.md) to configure it.

#### OAuth Passthrough

OAuth Passthrough lets users authenticate with their own Google OAuth identity through Grafana. This requires Grafana to be configured with a Google OAuth provider that has the `https://www.googleapis.com/auth/cloud-platform.read-only` scope. When enabled, the plugin forwards the user's browser OAuth token to GCP on each request.

> **Note:** When using OAuth Passthrough, you must provide a **Default Project ID** in the configuration since the plugin cannot auto-detect the project from forwarded credentials.

### Service Account Impersonation
You can also configure the plugin to use [service account impersonation](https://cloud.google.com/iam/docs/service-account-impersonation).
You need to ensure the service account used by this plugin has the `iam.serviceAccounts.getAccessToken` permission. This permission is in roles like the [Service Account Token Creator role](https://cloud.google.com/iam/docs/understanding-roles#iam.serviceAccountTokenCreator) (roles/iam.serviceAccountTokenCreator). Also, the service account impersonated
by this plugin needs cloud trace user and project list permissions.

> **Note:** Service account impersonation is available with JWT and GCE authentication types, but not with OAuth Passthrough.

### Universe Domain

If you are using a non-default universe (e.g., a sovereign cloud), you can configure the **Universe Domain** field in the data source settings. Leave it empty to use the default (`googleapis.com`).

### Grafana Configuration
1. With Grafana restarted, navigate to `Configuration -> Data sources` (or the route `/datasources`)
2. Click "Add data source"
3. Select "Google Cloud Trace"
4. Select the authentication type from the dropdown (JWT, GCE, Access Token, or OAuth Passthrough)
5. Provide the required credentials for your chosen authentication method
6. Optionally, configure the **Universe Domain** if you are using a non-default GCP environment
7. Optionally, configure the **Project List Filter** to restrict which projects appear in the project dropdown (see [Project List Filter](#project-list-filter) below)
8. Click "Save & test" to test that traces can be queried from Cloud Trace.

![image info](https://github.com/GoogleCloudPlatform/cloud-trace-data-source-plugin/blob/main/src/img/cloud_trace_config.png?raw=true)

### Project List Filter

If you have access to many GCP projects, you can restrict which projects appear in the project dropdown by configuring a **Project List Filter** in the data source settings.

Enter project IDs or regex patterns in the text area, one per line. Only projects matching at least one pattern will appear in the dropdown. Leave the field empty to show all projects (the default behavior).

Each line is treated as a [regular expression](https://developer.mozilla.org/en-US/docs/Web/JavaScript/Guide/Regular_expressions) anchored to the full project ID. For example:

| Pattern | Matches |
| --- | --- |
| `my-project-123` | Only the exact project `my-project-123` |
| `team-alpha-.*` | All projects starting with `team-alpha-` |
| `prod-.*-trace` | Projects like `prod-us-trace`, `prod-eu-trace`, etc. |

You can combine multiple patterns (one per line) to match the union of all patterns. If a pattern contains invalid regex syntax, it is treated as a literal string match.

### Trace to logs

You can navigate from a span in the trace view to logs relevant to that span, the same way as in trace data sources like Tempo. Requires Grafana 11.2 or later and a logs data source such as [Google Cloud Logging](https://grafana.com/grafana/plugins/googlecloud-logging-datasource/).

In the data source settings, under **Trace to logs**:

- **Data source**: the [Google Cloud Logging](https://grafana.com/grafana/plugins/googlecloud-logging-datasource/) data source the span links to. Picking one seeds sensible defaults for the options below (filter by trace ID on, time shifts `-5m`/`5m`); you can change them afterwards.
- **Span start/end time shift**: widen the logs search window relative to the span, e.g. `-1h` and `1h`. With both empty the window is the span's own duration, which is often only milliseconds.
- **Tags**: span tags to include in the logs query; the optional value renames the tag in the query.
- **Filter by trace ID / span ID**: restrict the logs query to the span's trace/span ID.
- **Use custom query**: write your own Cloud Logging query with the variables `${__span.traceId}`, `${__span.spanId}`, `${__span.tags.X}`, and `$__tags`, e.g. `trace="projects/my-project/traces/${__span.traceId}"`.

With this configured, spans in Explore and dashboard trace panels show a **Logs for this span** button.

> **Note:** the generated logs query is built only from the enabled filters, matching tags, and the custom query. If all of them are off/empty, the link produces an **empty query** and the logs panel returns everything (or nothing) in the span's time window. Keep at least **Filter by trace ID** enabled (the default when configuring through the UI), and when provisioning, set `filterByTraceID: true` explicitly.

Provisioning example:

```yaml
jsonData:
  tracesToLogsV2:
    datasourceUid: my-cloud-logging-datasource-uid
    spanStartTimeShift: "-5m"
    spanEndTimeShift: "5m"
    filterByTraceID: true
```

For the reverse direction (from a log entry to its trace), configure **Logs to traces** in the Google Cloud Logging data source settings.

## Usage

### Grafana Explore
1. After configuration, navigate to `Explore` (or the route `/explore`).
2. Select "Google Cloud Trace" from the dropdown list of datasources.
3. Select either `Filter` or `Trace ID` for the query type.
4. For `Trace ID` queries, simply enter in a trace ID to view the trace and its associated spans.
5. For `Filter` queries, enter any number of filters in the form of `[key]:[value]`. 
   Typically these filters are are used to match labels on the traces. These filters are additive.
   There are also a number of special user friendly keys you can use:
    - `RootSpan` matches any trace which contains the given root span name
    - `SpanName` matches any trace which contains the given span name
    - `HasLabel` matches any trace which contains the given label key
	- `MinLatency` matches any trace which has a latency greater than the given latency
	- `Version` matches any trace which contains the label `g.co/gae/app/version` with the given service version
	- `Service` matches any trace which contains the label `g.co/gae/app/module` with the given service name
	- `Status` matches any trace which contains the label `/http/status_code` with the given status
	- `URL` matches any trace which contains the label `/http/url` with the given url
	- `Method` matches any trace which contains the label `/http/method` with the given HTTP method

    After making a `Filter` query, a table will be displayed with all of the matching traces
    (Example: `http.scheme:http http.server_name:testserver MinLatency:500ms`)

### Span status and errors

The trace view marks spans as failed (error icon and `statusCode`/`statusMessage` fields) based on
the span's Cloud Trace labels:

- `/error/message` or `/error/name`: the span is marked as an error, using the label value as the
  status message
- `/http/status_code`, `http.status_code`, or `http.response.status_code`: the span is marked as an
  error for status codes >= 500 (>= 400 for client spans, per OpenTelemetry HTTP semantics)

If the span has a `/stacktrace` label, its value is shown in the span's "Stack Traces" section.

Note: the Cloud Trace v1 read API used by this plugin does not return span events (`TimeEvents`),
so events such as OpenTelemetry exceptions attached to a span cannot be displayed. Error details
are only available when they are present as span labels.

### Annotations

The plugin supports Grafana annotations. You can use trace queries as annotation sources to overlay trace data on your dashboards.

### Supported variables
The plugin currently supports variables for the GCP projects and a trace id. The project variable is a query one, and the trace id is a text or custom one.

## Development

### Prerequisites
- Node.js >= 20
- Go >= 1.25
- [Mage](https://magefile.org/)

### Building
```bash
# Frontend
yarn install
yarn build

# Backend
mage -v
```

### Testing
```bash
# Frontend tests
yarn test:ci

# Backend tests
go test ./pkg/...
```

## Licenses
Cloud Trace Logo (`src/img/logo.svg`) is from Google Cloud's [Official icons and sample diagrams](https://cloud.google.com/icons)

As commented, `JWTForm` and `JWTConfigEditor` are largely based on Apache-2.0 licensed [grafana-google-sdk-react](https://github.com/grafana/grafana-google-sdk-react/)
