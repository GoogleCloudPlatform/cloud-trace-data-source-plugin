# Changelog
## 1.5.0 (2026-11-27)
* **Span IDs are now 16-character lowercase hex** (the Cloud Trace v2, OpenTelemetry, and Cloud Logging `spanId` format) instead of decimal, and root spans no longer reference a parent span `0`. Trace-to-logs **Filter by span ID** now matches Cloud Logging entries. Update custom queries or workarounds that relied on decimal span IDs ([#24](https://github.com/GoogleCloudPlatform/cloud-trace-data-source-plugin/issues/24))
* Fix the trace list failing when Cloud Trace returns a trace without a root span, show `ListTraces` errors (for example a missing permission or project) instead of an empty table, and return up to 100 traces when max data points is 0
* GCE / Application Default Credentials: an explicit default project now also preselects that project in the query editor instead of the VM's metadata project ([#16](https://github.com/GoogleCloudPlatform/cloud-trace-data-source-plugin/issues/16)). A failed lookup of the VM's project is retried after a minute instead of being kept until the page is reloaded ([#117](https://github.com/GoogleCloudPlatform/cloud-trace-data-source-plugin/pull/117))
* Config editor: remove the plugin's duplicate default project and service account impersonation fields (the JWT and GCE settings already provide them) and fix the JWT key form not appearing after switching from GCE to JWT ([#113](https://github.com/GoogleCloudPlatform/cloud-trace-data-source-plugin/pull/113)). A data source with a stored access token but no authentication type now shows the **Access Token** settings, matching what the backend uses ([#117](https://github.com/GoogleCloudPlatform/cloud-trace-data-source-plugin/pull/117))
* Health check: explain how to set a default project instead of showing the raw `projects/` API error ([#114](https://github.com/GoogleCloudPlatform/cloud-trace-data-source-plugin/pull/114)). The health check now uses the same authentication settings as queries, and reports success when the project is reachable but has no traces from the last 30 days ([#117](https://github.com/GoogleCloudPlatform/cloud-trace-data-source-plugin/pull/117))
* Trace to logs: warn when Grafana's default tags would make Cloud Logging reject the logs query ([#115](https://github.com/GoogleCloudPlatform/cloud-trace-data-source-plugin/pull/115)), including when **Use custom query** is on but the query is empty. The settings and README now explain how to rename span tags to Cloud Logging fields, suggest exact `trace:"${__span.traceId}"` queries, and include a working provisioning example ([#117](https://github.com/GoogleCloudPlatform/cloud-trace-data-source-plugin/pull/117))
* Errors from Cloud Trace keep their status code and message but no longer include internal error details such as `DebugInfo` ([#117](https://github.com/GoogleCloudPlatform/cloud-trace-data-source-plugin/pull/117))
* Trace view: a span without an end time gets a duration of 0 instead of a large negative value ([#117](https://github.com/GoogleCloudPlatform/cloud-trace-data-source-plugin/pull/117))
* Upgrade grafana-plugin-sdk-go to v0.303.0 and patch HIGH advisories in build dependencies (nanoid, zod) ([#112](https://github.com/GoogleCloudPlatform/cloud-trace-data-source-plugin/pull/112))
* CI: build with Go 1.27.4 and Node 24, run frontend type checks and tests, and run the plugin validator's source-code checks with grafana.com's severities on pull requests, on pushes to `main`, and weekly. Fail when `yarn.lock` is out of date ([#112](https://github.com/GoogleCloudPlatform/cloud-trace-data-source-plugin/pull/112), [#117](https://github.com/GoogleCloudPlatform/cloud-trace-data-source-plugin/pull/117))
* Release workflow: pass the plugin signing token only to the signing step ([#117](https://github.com/GoogleCloudPlatform/cloud-trace-data-source-plugin/pull/117))
* Docs: correct the supported Grafana version (11.2.0+) and move the build instructions to CONTRIBUTING.md ([#117](https://github.com/GoogleCloudPlatform/cloud-trace-data-source-plugin/pull/117))
* Update dependencies

## 1.4.0 (2026-07-20)
* Add **Trace to logs** correlation: configure a logs data source (e.g. Google Cloud Logging), span time shifts, tags, and trace/span ID filters — or a custom query — in the data source settings, and spans in the trace view get a "Logs for this span" link (requires Grafana >= 11.2)
* Picking the Trace-to-logs data source seeds default settings (filter by trace ID on, span time shifts `-1h`/`1h`) so a fresh configuration generates a useful logs query instead of an empty one

## 1.3.5 (2026-07-14)
* Expose span status in the trace view: populate `kind`, `statusCode`, and `statusMessage` data frame fields, derived from Cloud Trace span labels (`/error/message`, `/error/name`, and HTTP status code labels per OpenTelemetry HTTP semantics)
* Mark failed spans with an `error: true` tag so they are highlighted with the error icon in the Grafana trace view
* Show the span's `/stacktrace` label in the "Stack Traces" section via the `stackTraces` field
* Document that the Cloud Trace v1 read API does not return span events (`TimeEvents`), so error details are only available from span labels

## 1.3.4 (2026-05-08)
* Upgrade @grafana/google-sdk from 0.0.3 to 0.3.5 to fix ConnectionConfig mutating React props ([#44](https://github.com/GoogleCloudPlatform/cloud-trace-data-source-plugin/issues/44))
* Upgrade @grafana/data, @grafana/runtime, @grafana/ui to ^11.0.0
* Raise minimum supported Grafana version to 11.2.0
* Update @types/react from ^17.0.2 to ^18.2.0 to match React 18 runtime
* Fix stale project ID persisting in query editor when switching between datasources

## 1.3.3 (2026-04-05)
* Add Project List Filter to restrict which projects appear in dropdowns using regex patterns
* Fix race condition in variable query where default project was set via unawaited promise

## 1.3.2 (2026-03-18)
* Fix project dropdown search failing with "contains global restriction" error
* Return error responses as JSON so Grafana displays actual error messages instead of generic "Unexpected error"
* Suppress duplicate error toast notifications — errors now only appear in the inline alert
* Fix inline error alert showing raw JSON instead of the error message text

## 1.3.1 (2026-03-15)
* Improve project dropdown performance with AsyncSelect and server-side search
* Cache default project list to reduce redundant backend requests
* Cap ListProjects results to 100 for performance
* Use Grafana UI components (Field, Input, Alert) in ConfigEditor and QueryEditor
* Fix log levels from Warn to Error for GCE and ListProjects errors
* Add request URL parsing for project search query support
* Add .gitattributes for GitHub language statistics
* Enhance filterQuery to skip empty traceID and missing projectId queries
* Add null safety guard in addLinksToTraceIdColumn
* Persist default query values via onChange in QueryEditor
* Fix resource leak: close trace client when resource manager init fails
* Fix direct props mutation in ConfigEditor service account impersonation

## 1.3.0 (2026-03-09)
* Support OAuth passthrough authentication
* Add universe domain support
* Add HTML error sanitization (frontend and backend)
* Add pagination to ListProjects using gRPC v3 Resource Manager client
* Add annotation support
* Improve error responses in CallResource (return proper HTTP status codes)
* Add authentication type selector dropdown in ConfigEditor
* Migrate build tooling from @grafana/toolkit to webpack
* Update dependencies

## 1.2.0 (2025-04-08)
* Support new Access Token auth type

## 1.1.0 (2023-10-07)
* Support service account impersonation
* Support the project and trace id variables
* Update dependencies

## 1.0.0 (2023-06-16)

Initial release.
