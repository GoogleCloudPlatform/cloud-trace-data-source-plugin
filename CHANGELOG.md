# Changelog
## Unreleased
* Add **Trace to logs** correlation: configure a logs data source (e.g. Google Cloud Logging), span time shifts, tags, and trace/span ID filters — or a custom query — in the data source settings, and spans in the trace view get a "Logs for this span" link (requires Grafana >= 11.2)

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
