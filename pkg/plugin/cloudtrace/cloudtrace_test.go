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

package cloudtrace_test

import (
	"encoding/json"
	"errors"
	"testing"

	"cloud.google.com/go/trace/apiv1/tracepb"
	"github.com/GoogleCloudPlatform/cloud-trace-data-source-plugin/pkg/plugin/cloudtrace"
	"github.com/stretchr/testify/require"
)

func TestGetTraceName(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name              string
		span              *tracepb.TraceSpan
		expectedTraceName string
	}{
		{
			name:              "Span with no labels or name",
			span:              &tracepb.TraceSpan{},
			expectedTraceName: "",
		},
		{
			name:              "Span with no labels",
			span:              &tracepb.TraceSpan{Name: "spanname"},
			expectedTraceName: "spanname",
		},
		{
			name: "Span with no expected service or method label",
			span: &tracepb.TraceSpan{
				Name:   "spanname",
				Labels: map[string]string{"service": "servicename", "method": "method name"},
			},
			expectedTraceName: "spanname",
		},
		{
			name: "Span with no service label",
			span: &tracepb.TraceSpan{
				Name:   "spanname",
				Labels: map[string]string{"/http/method": "GET"},
			},
			expectedTraceName: "HTTP GET spanname",
		},
		{
			name: "Span with no method label",
			span: &tracepb.TraceSpan{
				Name:   "spanname",
				Labels: map[string]string{"g.co/gae/app/module": "servicename"},
			},
			expectedTraceName: "servicename: spanname",
		},
		{
			name: "Span with OTEL method label",
			span: &tracepb.TraceSpan{
				Name:   "spanname",
				Labels: map[string]string{"http.method": "DELETE"},
			},
			expectedTraceName: "HTTP DELETE spanname",
		},
		{
			name: "Span with service and method labels",
			span: &tracepb.TraceSpan{
				Name: "spanname",
				Labels: map[string]string{
					"g.co/gae/app/module": "servicename",
					"/http/method":        "GET",
				},
			},
			expectedTraceName: "servicename: HTTP GET spanname",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := cloudtrace.GetTraceName(tc.span)

			require.Equal(t, tc.expectedTraceName, result)
		})
	}
}

func TestGetSpanOperationName(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name                      string
		span                      *tracepb.TraceSpan
		expectedSpanOperationName string
	}{
		{
			name:                      "Span with no labels or name",
			span:                      &tracepb.TraceSpan{},
			expectedSpanOperationName: "",
		},
		{
			name:                      "Span with no labels",
			span:                      &tracepb.TraceSpan{Name: "spanname"},
			expectedSpanOperationName: "spanname",
		},
		{
			name: "Span with no expected method label",
			span: &tracepb.TraceSpan{
				Name:   "spanname",
				Labels: map[string]string{"service": "servicename", "method": "method name"},
			},
			expectedSpanOperationName: "spanname",
		},
		{
			name: "Span with OTEL method label",
			span: &tracepb.TraceSpan{
				Name: "spanname",
				Labels: map[string]string{
					"http.method": "GET",
				},
			},
			expectedSpanOperationName: "HTTP GET spanname",
		},
		{
			name: "Span with Cloud Trace method label",
			span: &tracepb.TraceSpan{
				Name: "spanname",
				Labels: map[string]string{
					"/http/method": "GET",
				},
			},
			expectedSpanOperationName: "HTTP GET spanname",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := cloudtrace.GetSpanOperationName(tc.span)

			require.Equal(t, tc.expectedSpanOperationName, result)
		})
	}
}

func TestGetTags(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name                string
		span                *tracepb.TraceSpan
		expectedServiceTags []map[string]string
		expectedSpanTags    []map[string]string
		expectedError       error
	}{
		{
			name:                "Span with no labels",
			span:                &tracepb.TraceSpan{},
			expectedServiceTags: []map[string]string{},
			expectedSpanTags:    []map[string]string{},
			expectedError:       nil,
		},
		{
			name: "Span with span labels",
			span: &tracepb.TraceSpan{
				Labels: map[string]string{
					"key1": "value1",
					"key2": "value2",
				},
			},
			expectedServiceTags: []map[string]string{},
			expectedSpanTags: []map[string]string{
				{"key": "key1", "value": "value1"},
				{"key": "key2", "value": "value2"},
			},
			expectedError: nil,
		},
		{
			name: "Span with service labels",
			span: &tracepb.TraceSpan{
				Labels: map[string]string{
					"service.name":    "servicename",
					"service.version": "100",
				},
			},
			expectedServiceTags: []map[string]string{
				{"key": "service.name", "value": "servicename"},
				{"key": "service.version", "value": "100"},
			},
			expectedSpanTags: []map[string]string{},
			expectedError:    nil,
		},
		{
			name: "Span with GAE service labels",
			span: &tracepb.TraceSpan{
				Labels: map[string]string{
					"g.co/gae/app/module":  "servicename",
					"g.co/gae/app/version": "100",
				},
			},
			expectedServiceTags: []map[string]string{
				{"key": "g.co/gae/app/module", "value": "servicename"},
				{"key": "g.co/gae/app/version", "value": "100"},
			},
			expectedSpanTags: []map[string]string{},
			expectedError:    nil,
		},
		{
			name: "Span with service and GAE service labels",
			span: &tracepb.TraceSpan{
				Labels: map[string]string{
					"service.name":         "servicename",
					"service.version":      "100",
					"g.co/gae/app/module":  "servicename",
					"g.co/gae/app/version": "100",
				},
			},
			expectedServiceTags: []map[string]string{
				{"key": "service.name", "value": "servicename"},
				{"key": "service.version", "value": "100"},
				{"key": "g.co/gae/app/module", "value": "servicename"},
				{"key": "g.co/gae/app/version", "value": "100"},
			},
			expectedSpanTags: []map[string]string{},
			expectedError:    nil,
		},
		{
			name: "Span with all labels",
			span: &tracepb.TraceSpan{
				Labels: map[string]string{
					"key1":                 "value1",
					"key2":                 "value2",
					"service.name":         "servicename",
					"service.version":      "100",
					"g.co/gae/app/module":  "servicename",
					"g.co/gae/app/version": "100",
				},
			},
			expectedServiceTags: []map[string]string{
				{"key": "service.name", "value": "servicename"},
				{"key": "service.version", "value": "100"},
				{"key": "g.co/gae/app/module", "value": "servicename"},
				{"key": "g.co/gae/app/version", "value": "100"},
			},
			expectedSpanTags: []map[string]string{
				{"key": "key1", "value": "value1"},
				{"key": "key2", "value": "value2"},
			},
			expectedError: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			serviceTags, spanTags, error := cloudtrace.GetTags(tc.span)

			if tc.expectedError != nil {
				require.ErrorIs(t, error, tc.expectedError)
			} else {
				require.Nil(t, error)
			}

			var serviceTagsMap []map[string]string
			err := json.Unmarshal(serviceTags, &serviceTagsMap)
			require.NoError(t, err)
			var spanTagsMap []map[string]string
			err = json.Unmarshal(spanTags, &spanTagsMap)
			require.NoError(t, err)
			require.ElementsMatch(t, tc.expectedServiceTags, serviceTagsMap)
			require.ElementsMatch(t, tc.expectedSpanTags, spanTagsMap)
		})
	}
}

func TestGetListTracesFilter(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name           string
		queryText      string
		expectedFilter string
		expectedErr    error
	}{
		{
			name:           "Query text with bad filter",
			queryText:      "badfilter",
			expectedFilter: "",
			expectedErr:    errors.New("bad filter [badfilter]. Must be in form [key]:[value]"),
		},
		{
			name:           "Query text with good and bad filter parts",
			queryText:      "LABEL:latency:100ms badfilter",
			expectedFilter: "",
			expectedErr:    errors.New("bad filter [badfilter]. Must be in form [key]:[value]"),
		},
		{
			name:           "Query text with bad LABEL filter",
			queryText:      "LABEL:badfilter",
			expectedFilter: "",
			expectedErr:    errors.New("bad filter [LABEL:badfilter]. Must be in form LABEL:[key]:[value]"),
		},
		{
			name:           "Query text with good and bad LABEL filter",
			queryText:      "LABEL:key1:value1 LABEL:badfilter",
			expectedFilter: "",
			expectedErr:    errors.New("bad filter [LABEL:badfilter]. Must be in form LABEL:[key]:[value]"),
		},
		{
			name:           "Query text with RootSpan filter",
			queryText:      "RootSpan:rootspan1",
			expectedFilter: "root:rootspan1",
			expectedErr:    nil,
		},
		{
			name:           "Query text with SpanName filter",
			queryText:      "SpanName:span1",
			expectedFilter: "span:span1",
			expectedErr:    nil,
		},
		{
			name:           "Query text with HasLabel filter",
			queryText:      "HasLabel:key1",
			expectedFilter: "label:key1",
			expectedErr:    nil,
		},
		{
			name:           "Query text with MinLatency filter",
			queryText:      "MinLatency:100ms",
			expectedFilter: "latency:100ms",
			expectedErr:    nil,
		},
		{
			name:           "Query text with URL filter",
			queryText:      "URL:http://www.test.com",
			expectedFilter: "url:http://www.test.com",
			expectedErr:    nil,
		},
		{
			name:           "Query text with Method filter",
			queryText:      "Method:GET",
			expectedFilter: "method:GET",
			expectedErr:    nil,
		},
		{
			name:           "Query text with Version filter",
			queryText:      "Version:1.0.0",
			expectedFilter: "g.co/gae/app/version:1.0.0",
			expectedErr:    nil,
		},
		{
			name:           "Query text with Service filter",
			queryText:      "Service:servicename",
			expectedFilter: "g.co/gae/app/module:servicename",
			expectedErr:    nil,
		},
		{
			name:           "Query text with Status filter",
			queryText:      "Status:200",
			expectedFilter: "/http/status_code:200",
			expectedErr:    nil,
		},
		{
			name:           "Query text with LABEL filter",
			queryText:      "LABEL:key1:value1",
			expectedFilter: "key1:value1",
			expectedErr:    nil,
		},
		{
			name:           "Query text with key value filter",
			queryText:      "key1:value1",
			expectedFilter: "key1:value1",
			expectedErr:    nil,
		},
		{
			name:           "Query text with special + char on value",
			queryText:      "key1:+value1",
			expectedFilter: "+key1:value1",
			expectedErr:    nil,
		},
		{
			name:           "Query text with special ^ char on value",
			queryText:      "key1:^value1",
			expectedFilter: "^key1:value1",
			expectedErr:    nil,
		},
		{
			name:           "Query text with special ^+ chars on value",
			queryText:      "key1:^+value1",
			expectedFilter: "+^key1:value1",
			expectedErr:    nil,
		},
		{
			name:           "Query text with special +^ chars on value",
			queryText:      "key1:+^value1",
			expectedFilter: "+^key1:value1",
			expectedErr:    nil,
		},
		{
			name:           "Query text with special + char on short value",
			queryText:      "key1:+v",
			expectedFilter: "+key1:v",
			expectedErr:    nil,
		},
		{
			name:           "Query text with special ^ char on short value",
			queryText:      "key1:^v",
			expectedFilter: "^key1:v",
			expectedErr:    nil,
		},
		{
			name:           "Query text with special ^+ char on short value",
			queryText:      "key1:^+v",
			expectedFilter: "+^key1:v",
			expectedErr:    nil,
		},
		{
			name:           "Query text with special +^ char on short value",
			queryText:      "key1:+^v",
			expectedFilter: "+^key1:v",
			expectedErr:    nil,
		},
		{
			name:           "Query text empty value",
			queryText:      "key1:",
			expectedFilter: "key1:",
			expectedErr:    nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := cloudtrace.GetListTracesFilter(tc.queryText)

			if tc.expectedErr == nil {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tc.expectedErr.Error())
			}

			require.Equal(t, tc.expectedFilter, result)
		})
	}
}

func TestGetStatus(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name            string
		span            *tracepb.TraceSpan
		expectedCode    int64
		expectedMessage string
	}{
		{
			name:            "Span with no labels",
			span:            &tracepb.TraceSpan{},
			expectedCode:    cloudtrace.StatusCodeUnset,
			expectedMessage: "",
		},
		{
			name: "Span with error message label",
			span: &tracepb.TraceSpan{
				Labels: map[string]string{
					"/error/message": "something went wrong",
					"/error/name":    "RuntimeError",
				},
			},
			expectedCode:    cloudtrace.StatusCodeError,
			expectedMessage: "something went wrong",
		},
		{
			name: "Span with only error name label",
			span: &tracepb.TraceSpan{
				Labels: map[string]string{
					"/error/name": "RuntimeError",
				},
			},
			expectedCode:    cloudtrace.StatusCodeError,
			expectedMessage: "RuntimeError",
		},
		{
			name: "Server span with HTTP 500",
			span: &tracepb.TraceSpan{
				Kind: tracepb.TraceSpan_RPC_SERVER,
				Labels: map[string]string{
					"/http/status_code": "500",
				},
			},
			expectedCode:    cloudtrace.StatusCodeError,
			expectedMessage: "HTTP 500",
		},
		{
			name: "Server span with HTTP 404 is not an error",
			span: &tracepb.TraceSpan{
				Kind: tracepb.TraceSpan_RPC_SERVER,
				Labels: map[string]string{
					"/http/status_code": "404",
				},
			},
			expectedCode:    cloudtrace.StatusCodeUnset,
			expectedMessage: "",
		},
		{
			name: "Client span with HTTP 404 is an error",
			span: &tracepb.TraceSpan{
				Kind: tracepb.TraceSpan_RPC_CLIENT,
				Labels: map[string]string{
					"/http/status_code": "404",
				},
			},
			expectedCode:    cloudtrace.StatusCodeError,
			expectedMessage: "HTTP 404",
		},
		{
			name: "Span with OTel status code label",
			span: &tracepb.TraceSpan{
				Labels: map[string]string{
					"http.status_code": "503",
				},
			},
			expectedCode:    cloudtrace.StatusCodeError,
			expectedMessage: "HTTP 503",
		},
		{
			name: "Span with new OTel semconv status code label",
			span: &tracepb.TraceSpan{
				Labels: map[string]string{
					"http.response.status_code": "502",
				},
			},
			expectedCode:    cloudtrace.StatusCodeError,
			expectedMessage: "HTTP 502",
		},
		{
			name: "Span with successful HTTP status",
			span: &tracepb.TraceSpan{
				Labels: map[string]string{
					"/http/status_code": "200",
				},
			},
			expectedCode:    cloudtrace.StatusCodeUnset,
			expectedMessage: "",
		},
		{
			name: "Span with unparsable HTTP status",
			span: &tracepb.TraceSpan{
				Labels: map[string]string{
					"/http/status_code": "abc",
				},
			},
			expectedCode:    cloudtrace.StatusCodeUnset,
			expectedMessage: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			code, message := cloudtrace.GetStatus(tc.span)

			require.Equal(t, tc.expectedCode, code)
			require.Equal(t, tc.expectedMessage, message)
		})
	}
}

func TestGetSpanKind(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name         string
		span         *tracepb.TraceSpan
		expectedKind string
	}{
		{
			name:         "Unspecified span kind",
			span:         &tracepb.TraceSpan{},
			expectedKind: "",
		},
		{
			name:         "Server span",
			span:         &tracepb.TraceSpan{Kind: tracepb.TraceSpan_RPC_SERVER},
			expectedKind: "server",
		},
		{
			name:         "Client span",
			span:         &tracepb.TraceSpan{Kind: tracepb.TraceSpan_RPC_CLIENT},
			expectedKind: "client",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.expectedKind, cloudtrace.GetSpanKind(tc.span))
		})
	}
}

func TestGetStackTraces(t *testing.T) {
	t.Parallel()

	t.Run("Span without stack trace", func(t *testing.T) {
		require.Nil(t, cloudtrace.GetStackTraces(&tracepb.TraceSpan{}))
	})

	t.Run("Span with stack trace", func(t *testing.T) {
		span := &tracepb.TraceSpan{
			Labels: map[string]string{
				"/stacktrace": `{"stack_frame":[{"method_name":"main"}]}`,
			},
		}

		stackTraces := cloudtrace.GetStackTraces(span)
		require.NotNil(t, stackTraces)

		var traces []string
		require.NoError(t, json.Unmarshal(*stackTraces, &traces))
		require.Equal(t, []string{`{"stack_frame":[{"method_name":"main"}]}`}, traces)
	})
}

func TestGetTagsErrorSpan(t *testing.T) {
	t.Parallel()

	span := &tracepb.TraceSpan{
		Labels: map[string]string{
			"/error/message": "something went wrong",
		},
	}

	_, spanTags, err := cloudtrace.GetTags(span)
	require.NoError(t, err)

	var spanTagsMap []map[string]any
	require.NoError(t, json.Unmarshal(spanTags, &spanTagsMap))
	require.ElementsMatch(t, []map[string]any{
		{"key": "/error/message", "value": "something went wrong"},
		{"key": "error", "value": true},
	}, spanTagsMap)
}
