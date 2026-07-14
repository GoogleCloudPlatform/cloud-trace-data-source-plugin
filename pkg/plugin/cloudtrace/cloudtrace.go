// Copyright 2023 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cloudtrace

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"cloud.google.com/go/trace/apiv1/tracepb"
)

const (
	servicePrefix        = "service."
	gaeServicePrefix     = "g.co/gae/app/"
	otelServiceKey       = "service.name"
	gaeServiceKey        = "g.co/gae/app/module"
	gaeServiceVersionKey = "g.co/gae/app/version"
	otelMethodKey        = "http.method"
	cloudTraceMethodKey  = "/http/method"

	errorMessageKey        = "/error/message"
	errorNameKey           = "/error/name"
	cloudTraceStatusKey    = "/http/status_code"
	otelStatusKey          = "http.status_code"
	otelHTTPResponseStatus = "http.response.status_code"
	stackTraceKey          = "/stacktrace"
	errorTagKey            = "error"
)

// Span status codes as expected by the Grafana trace schema,
// following the OpenTelemetry convention
const (
	StatusCodeUnset int64 = 0
	StatusCodeError int64 = 2
)

// Regex for individual filters within query text
var re = regexp.MustCompile(`(?:[^\s"]+|"(?:\\"|[^"])*")+`)

// TimeRange holds both a from and to time
type TimeRange struct {
	From time.Time
	To   time.Time
}

// GetServiceName returns the service name for the span
func GetServiceName(span *tracepb.TraceSpan) string {
	labels := span.GetLabels()

	// In both cases treating "not existing" and "empty value" the same
	serviceName := labels[otelServiceKey]
	if serviceName == "" {
		serviceName = labels[gaeServiceKey]
	}

	return serviceName
}

// GetTraceName gets the name, service label value, and method label value
// for the span and combines them to create a descriptive name
func GetTraceName(span *tracepb.TraceSpan) string {
	namePart := span.GetName()

	servicePart := GetServiceName(span)
	if servicePart != "" {
		servicePart = fmt.Sprintf("%s: ", servicePart)
	}

	methodPart := getHTTPMethod(span)
	if methodPart != "" {
		methodPart = fmt.Sprintf("HTTP %s ", methodPart)
	}

	return fmt.Sprintf("%s%s%s", servicePart, methodPart, namePart)
}

// GetSpanOperationName gets the name and method label value
// for the span and combines them to create a descriptive name
func GetSpanOperationName(span *tracepb.TraceSpan) string {
	namePart := span.GetName()

	methodPart := getHTTPMethod(span)
	if methodPart != "" {
		methodPart = fmt.Sprintf("HTTP %s ", methodPart)
	}

	return fmt.Sprintf("%s%s", methodPart, namePart)
}

// GetTags converts Google Trace labels to Grafana service and span tags
func GetTags(span *tracepb.TraceSpan) (serviceTags json.RawMessage, spanTags json.RawMessage, err error) {
	spanLabels := span.GetLabels()
	serviceTagsMapArray := []map[string]any{}
	spanTagsMapArray := []map[string]any{}
	for key, value := range spanLabels {
		if strings.HasPrefix(key, servicePrefix) || strings.HasPrefix(key, gaeServicePrefix) {
			serviceTagsMapArray = append(serviceTagsMapArray, map[string]any{"key": key, "value": value})
		} else {
			spanTagsMapArray = append(spanTagsMapArray, map[string]any{"key": key, "value": value})
		}
	}

	// Grafana's trace view highlights spans that carry an `error: true` tag,
	// so mark failed spans for versions that don't key off the statusCode field
	if code, _ := GetStatus(span); code == StatusCodeError {
		if _, ok := spanLabels[errorTagKey]; !ok {
			spanTagsMapArray = append(spanTagsMapArray, map[string]any{"key": errorTagKey, "value": true})
		}
	}

	serviceTags, err = json.Marshal(serviceTagsMapArray)
	if err != nil {
		return nil, nil, err
	}

	spanTags, err = json.Marshal(spanTagsMapArray)
	if err != nil {
		return nil, nil, err
	}

	return serviceTags, spanTags, nil
}

// GetStatus derives a Grafana span status code and message from the span's
// labels. The Cloud Trace v1 read API has no structured status field, so
// error information is only available through well-known labels: the
// canonical `/error/*` labels, or an HTTP status code label indicating a
// failed request.
func GetStatus(span *tracepb.TraceSpan) (int64, string) {
	labels := span.GetLabels()

	errMsg := labels[errorMessageKey]
	errName := labels[errorNameKey]
	if errMsg != "" || errName != "" {
		if errMsg == "" {
			errMsg = errName
		}
		return StatusCodeError, errMsg
	}

	httpStatus := labels[cloudTraceStatusKey]
	if httpStatus == "" {
		httpStatus = labels[otelStatusKey]
	}
	if httpStatus == "" {
		httpStatus = labels[otelHTTPResponseStatus]
	}
	if code, err := strconv.Atoi(httpStatus); err == nil {
		// Per OpenTelemetry HTTP semantics, 4xx responses are errors for
		// client spans but not for server spans
		threshold := 500
		if span.GetKind() == tracepb.TraceSpan_RPC_CLIENT {
			threshold = 400
		}
		if code >= threshold {
			return StatusCodeError, fmt.Sprintf("HTTP %d", code)
		}
	}

	return StatusCodeUnset, ""
}

// GetSpanKind maps the Cloud Trace span kind to the values expected by the
// Grafana trace schema
func GetSpanKind(span *tracepb.TraceSpan) string {
	switch span.GetKind() {
	case tracepb.TraceSpan_RPC_SERVER:
		return "server"
	case tracepb.TraceSpan_RPC_CLIENT:
		return "client"
	default:
		return ""
	}
}

// GetStackTraces returns the span's `/stacktrace` label as the JSON string
// array expected by the Grafana trace schema, or nil if the span has none
func GetStackTraces(span *tracepb.TraceSpan) *json.RawMessage {
	st := span.GetLabels()[stackTraceKey]
	if st == "" {
		return nil
	}

	b, err := json.Marshal([]string{st})
	if err != nil {
		return nil
	}

	raw := json.RawMessage(b)
	return &raw
}

// GetListTracesFilter takes the raw query text from a user and converts it
// to a filter string as expected by the Cloud Trace API
func GetListTracesFilter(queryText string) (string, error) {
	// Collect all filter parts from the query text
	qTFilters := re.FindAllString(queryText, -1)

	filters := make([]string, 0, len(qTFilters))
	for _, qTFilter := range qTFilters {
		key, value, err := getFilterKeyValue(qTFilter)
		if err != nil {
			return "", err
		}

		filters = append(filters, fmt.Sprintf("%s:%s", key, value))
	}

	return strings.Join(filters, " "), nil
}

func getHTTPMethod(span *tracepb.TraceSpan) string {
	labels := span.GetLabels()

	// In both cases treating "not existing" and "empty value" the same
	httpMethod := labels[otelMethodKey]
	if httpMethod == "" {
		httpMethod = labels[cloudTraceMethodKey]
	}

	return httpMethod
}

func getFilterKeyValue(qTFilter string) (key string, value string, err error) {
	// Filter part must be in form [key]:[value] from user
	qTFilterParts := strings.SplitN(qTFilter, ":", 2)
	if len(qTFilterParts) != 2 {
		return "", "", fmt.Errorf("bad filter [%s]. Must be in form [key]:[value]", qTFilter)
	}

	key = qTFilterParts[0]
	value = qTFilterParts[1]

	// OR for generic labels filter must be in form LABEL:[key]:[value] from user
	if strings.ToLower(key) == "label" {
		qTFilterParts := strings.SplitN(value, ":", 2)

		if len(qTFilterParts) != 2 {
			return "", "", fmt.Errorf("bad filter [%s]. Must be in form LABEL:[key]:[value]", qTFilter)
		}

		// Cloud Trace API should not have "LABEL:" in filter
		key = qTFilterParts[0]
		value = qTFilterParts[1]
	}

	// Convert key to Cloud Trace API expected form if needed
	switch key {
	case "RootSpan":
		key = "root"
	case "SpanName":
		key = "span"
	case "HasLabel":
		key = "label"
	case "MinLatency":
		key = "latency"
	case "URL":
		key = "url"
	case "Method":
		key = "method"
		// Currently matches the Google Cloud Trace UI filter, but ignores "service.version" matches
	case "Version":
		key = gaeServiceVersionKey
		// Currently matches the Google Cloud Trace UI filter, but ignores "service.name" matches
	case "Service":
		key = gaeServiceKey
	case "Status":
		key = "/http/status_code"
	}

	// If the value has less than 2 chars, no need to check for special filter chars
	if len(value) < 2 {
		return key, value, nil
	}

	firstChar := string(value[0])
	secondChar := string(value[1])

	// Move specials chars from the front of value to key for Google Cloud Trace compatibility
	if (secondChar == "^" && firstChar == "+") || (secondChar == "+" && firstChar == "^") {
		key = fmt.Sprintf("+^%s", key)
		value = value[2:]
	} else if firstChar == "+" || firstChar == "^" {
		key = fmt.Sprintf("%s%s", firstChar, key)
		value = value[1:]
	}

	return key, value, nil
}
