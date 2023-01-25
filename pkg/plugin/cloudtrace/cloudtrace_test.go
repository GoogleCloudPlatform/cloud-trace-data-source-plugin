package cloudtrace_test

import (
	"errors"
	"testing"

	"cloud.google.com/go/trace/apiv1/tracepb"
	"github.com/observiq/cloud-trace-grafana-ds/pkg/plugin/cloudtrace"
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
			expectedErr:    errors.New("Bad filter [badfilter]. Must be in form [key]:[value]"),
		},
		{
			name:           "Query text with good and bad filter parts",
			queryText:      "LABEL:latency:100ms badfilter",
			expectedFilter: "",
			expectedErr:    errors.New("Bad filter [badfilter]. Must be in form [key]:[value]"),
		},
		{
			name:           "Query text with bad LABEL filter",
			queryText:      "LABEL:badfilter",
			expectedFilter: "",
			expectedErr:    errors.New("Bad filter [LABEL:badfilter]. Must be in form LABEL:[key]:[value]"),
		},
		{
			name:           "Query text with good and bad LABEL filter",
			queryText:      "LABEL:key1:value1 LABEL:badfilter",
			expectedFilter: "",
			expectedErr:    errors.New("Bad filter [LABEL:badfilter]. Must be in form LABEL:[key]:[value]"),
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
		// Uncertain of this test. May need to use different label key
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
