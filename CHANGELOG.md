# Changelog
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
