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

package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"cloud.google.com/go/trace/apiv1/tracepb"
	cloudtrace "github.com/GoogleCloudPlatform/cloud-trace-data-source-plugin/pkg/plugin/cloudtrace"
	"github.com/GoogleCloudPlatform/cloud-trace-data-source-plugin/pkg/plugin/mocks"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// This is where the tests for the datasource backend live.
func TestQueryData(t *testing.T) {
	ds := CloudTraceDatasource{}

	resp, err := ds.QueryData(
		context.Background(),
		&backend.QueryDataRequest{
			Queries: []backend.DataQuery{
				{RefID: "A"},
			},
		},
	)
	if err != nil {
		t.Error(err)
	}

	if len(resp.Responses) != 1 {
		t.Fatal("QueryData must return a response")
	}
}

func TestQueryData_InvalidJSON(t *testing.T) {
	client := mocks.NewAPI(t)
	ds := CloudTraceDatasource{
		client: client,
	}
	refID := "test"
	resp, err := ds.QueryData(context.Background(), &backend.QueryDataRequest{
		Queries: []backend.DataQuery{
			{
				JSON:  []byte(`Not JSON`),
				RefID: refID,
			},
		},
	})

	require.NoError(t, err)
	require.Error(t, resp.Responses[refID].Error)
	require.Nil(t, resp.Responses[refID].Frames)
	client.AssertExpectations(t)
}

func TestQueryData_ListTracesGCPError(t *testing.T) {
	to := time.Now()
	from := to.Add(-1 * time.Hour)
	expectedErr := errors.New("something was wrong with the request")

	client := mocks.NewAPI(t)
	client.On("ListTraces", mock.Anything, &cloudtrace.TracesQuery{
		ProjectID: "testing",
		Filter:    `resource.type:"testing"`,
		Limit:     20,
		TimeRange: cloudtrace.TimeRange{
			From: from,
			To:   to,
		},
	}).Return(nil, expectedErr)

	ds := CloudTraceDatasource{
		client: client,
	}
	refID := "test"
	resp, err := ds.QueryData(context.Background(), &backend.QueryDataRequest{
		Queries: []backend.DataQuery{
			{
				JSON:  []byte(`{"projectId": "testing", "queryText": "resource.type:\"testing\""}`),
				RefID: refID,
				TimeRange: backend.TimeRange{
					From: from,
					To:   to,
				},
				MaxDataPoints: 20,
			},
		},
	})

	require.NoError(t, err)
	require.ErrorContains(t, resp.Responses[refID].Error, expectedErr.Error())
	require.Nil(t, resp.Responses[refID].Frames)
	client.AssertExpectations(t)
}

func TestQueryData_GetTraceGCPError(t *testing.T) {
	to := time.Now()
	from := to.Add(-1 * time.Hour)
	expectedErr := errors.New("something was wrong with the request")

	client := mocks.NewAPI(t)
	client.On("GetTrace", mock.Anything, &cloudtrace.TraceQuery{
		ProjectID: "testing",
		TraceID:   "123",
	}).Return(nil, expectedErr)

	ds := CloudTraceDatasource{
		client: client,
	}
	refID := "test"
	resp, err := ds.QueryData(context.Background(), &backend.QueryDataRequest{
		Queries: []backend.DataQuery{
			{
				JSON:  []byte(`{"projectId": "testing", "queryType": "traceID", "traceId": "123", "queryText": "resource.type:\"testing\""}`),
				RefID: refID,
				TimeRange: backend.TimeRange{
					From: from,
					To:   to,
				},
				MaxDataPoints: 20,
			},
		},
	})

	require.NoError(t, err)
	require.ErrorContains(t, resp.Responses[refID].Error, expectedErr.Error())
	require.Nil(t, resp.Responses[refID].Frames)
	client.AssertExpectations(t)
}

func TestQueryData_BadFilter(t *testing.T) {
	to := time.Now()
	from := to.Add(-1 * time.Hour)

	client := mocks.NewAPI(t)
	ds := CloudTraceDatasource{
		client: client,
	}
	refID := "test"
	resp, err := ds.QueryData(context.Background(), &backend.QueryDataRequest{
		Queries: []backend.DataQuery{
			{
				JSON:  []byte(`{"projectId": "testing", "traceId": "123", "queryText": "resource.type.testing"}`),
				RefID: refID,
				TimeRange: backend.TimeRange{
					From: from,
					To:   to,
				},
				MaxDataPoints: 20,
			},
		},
	})

	require.NoError(t, err)
	require.ErrorContains(t, resp.Responses[refID].Error, "bad filter [resource.type.testing]. Must be in form [key]:[value]")
	require.Nil(t, resp.Responses[refID].Frames)
	client.AssertExpectations(t)
}

func TestQueryData_SingleTraceSpans(t *testing.T) {
	to := time.Now()
	from := to.Add(-1 * time.Hour)
	traceID := "123"
	startTime := timestamppb.New(time.UnixMilli(1660920349373))
	endTime := timestamppb.New(time.UnixMilli(1660920349374))

	spans := []*tracepb.TraceSpan{
		{
			SpanId:    1,
			Kind:      tracepb.TraceSpan_RPC_SERVER,
			Name:      "spanName",
			StartTime: startTime,
			EndTime:   endTime,
			Labels:    map[string]string{"key1": "value1"},
		},
	}
	trace := tracepb.Trace{
		ProjectId: "testProject",
		TraceId:   traceID,
		Spans:     spans,
	}

	client := mocks.NewAPI(t)
	client.On("GetTrace", mock.Anything, &cloudtrace.TraceQuery{
		ProjectID: "testing",
		TraceID:   traceID,
	}).Return(&trace, nil)
	client.On("Close").Return(nil)

	ds := CloudTraceDatasource{
		client: client,
	}
	refID := "test"
	resp, err := ds.QueryData(context.Background(), &backend.QueryDataRequest{
		Queries: []backend.DataQuery{
			{
				JSON:  []byte(`{"projectId": "testing", "queryType": "traceID", "traceId": "123", "queryText": "resource.type:\"testing\""}`),
				RefID: refID,
				TimeRange: backend.TimeRange{
					From: from,
					To:   to,
				},
				MaxDataPoints: 20,
			},
		},
	})
	ds.Dispose()
	require.NoError(t, err)
	require.Len(t, resp.Responses[refID].Frames, 1)

	traceFrame := resp.Responses[refID].Frames[0]
	require.Equal(t, traceID, traceFrame.Name)
	require.Len(t, traceFrame.Fields, 13)
	require.Equal(t, data.VisTypeTrace, string(traceFrame.Meta.PreferredVisualization))

	expectedFrame := []byte(`{"schema":{"name":"123","meta":{"typeVersion":[0,0],"preferredVisualisationType":"trace"},"fields":[{"name":"traceID","type":"string","typeInfo":{"frame":"string"}},{"name":"parentSpanID","type":"string","typeInfo":{"frame":"string"}},{"name":"spanID","type":"string","typeInfo":{"frame":"string"}},{"name":"serviceName","type":"string","typeInfo":{"frame":"string"}},{"name":"operationName","type":"string","typeInfo":{"frame":"string"}},{"name":"serviceTags","type":"other","typeInfo":{"frame":"json.RawMessage"}},{"name":"tags","type":"other","typeInfo":{"frame":"json.RawMessage"}},{"name":"startTime","type":"time","typeInfo":{"frame":"time.Time"}},{"name":"duration","type":"number","typeInfo":{"frame":"float64"}},{"name":"kind","type":"string","typeInfo":{"frame":"string"}},{"name":"statusCode","type":"number","typeInfo":{"frame":"int64"}},{"name":"statusMessage","type":"string","typeInfo":{"frame":"string"}},{"name":"stackTraces","type":"other","typeInfo":{"frame":"json.RawMessage","nullable":true}}]},"data":{"values":[["123"],[""],["0000000000000001"],[""],["spanName"],[[]],[[{"key":"key1","value":"value1"}]],[1660920349373],[1],["server"],[0],[""],[null]]}}`)

	serializedFrame, err := traceFrame.MarshalJSON()
	require.NoError(t, err)
	require.Equal(t, string(expectedFrame), string(serializedFrame))

	client.AssertExpectations(t)
}

func TestCreateTraceSpanFrame_HexSpanIDs(t *testing.T) {
	startTime := timestamppb.New(time.UnixMilli(1660920349373))
	endTime := timestamppb.New(time.UnixMilli(1660920349375))

	trace := &tracepb.Trace{
		ProjectId: "testProject",
		TraceId:   "0123456789abcdef0123456789abcdef",
		Spans: []*tracepb.TraceSpan{
			{
				SpanId:       0x4a,
				ParentSpanId: 0,
				Name:         "root",
				StartTime:    startTime,
				EndTime:      endTime,
			},
			{
				SpanId:       0xfedcba9876543210, // > math.MaxInt64
				ParentSpanId: 0x4a,
				Name:         "child",
				StartTime:    startTime,
				EndTime:      endTime,
			},
		},
	}

	frame := createTraceSpanFrame(trace)
	fieldsByName := map[string]*data.Field{}
	for _, field := range frame.Fields {
		fieldsByName[field.Name] = field
	}

	require.Equal(t, "000000000000004a", fieldsByName["spanID"].At(0))
	require.Equal(t, "", fieldsByName["parentSpanID"].At(0))
	require.Equal(t, "fedcba9876543210", fieldsByName["spanID"].At(1))
	require.Equal(t, "000000000000004a", fieldsByName["parentSpanID"].At(1))
}

func TestQueryData_TraceSpansWithError(t *testing.T) {
	to := time.Now()
	from := to.Add(-1 * time.Hour)
	traceID := "456"
	startTime := timestamppb.New(time.UnixMilli(1660920349373))
	endTime := timestamppb.New(time.UnixMilli(1660920349374))
	stackTrace := `{"stack_frame":[{"method_name":"main"}]}`

	spans := []*tracepb.TraceSpan{
		{
			SpanId:    1,
			Kind:      tracepb.TraceSpan_RPC_CLIENT,
			Name:      "failedSpan",
			StartTime: startTime,
			EndTime:   endTime,
			Labels: map[string]string{
				"/error/message": "something went wrong",
				"/stacktrace":    stackTrace,
			},
		},
	}
	trace := tracepb.Trace{
		ProjectId: "testProject",
		TraceId:   traceID,
		Spans:     spans,
	}

	client := mocks.NewAPI(t)
	client.On("GetTrace", mock.Anything, &cloudtrace.TraceQuery{
		ProjectID: "testing",
		TraceID:   traceID,
	}).Return(&trace, nil)
	client.On("Close").Return(nil)

	ds := CloudTraceDatasource{
		client: client,
	}
	refID := "test"
	resp, err := ds.QueryData(context.Background(), &backend.QueryDataRequest{
		Queries: []backend.DataQuery{
			{
				JSON:  []byte(`{"projectId": "testing", "queryType": "traceID", "traceId": "456"}`),
				RefID: refID,
				TimeRange: backend.TimeRange{
					From: from,
					To:   to,
				},
				MaxDataPoints: 20,
			},
		},
	})
	ds.Dispose()
	require.NoError(t, err)
	require.Len(t, resp.Responses[refID].Frames, 1)

	traceFrame := resp.Responses[refID].Frames[0]
	fieldsByName := map[string]*data.Field{}
	for _, field := range traceFrame.Fields {
		fieldsByName[field.Name] = field
	}

	require.Equal(t, int64(2), fieldsByName["statusCode"].At(0))
	require.Equal(t, "something went wrong", fieldsByName["statusMessage"].At(0))
	require.Equal(t, "client", fieldsByName["kind"].At(0))

	stackTraces := fieldsByName["stackTraces"].At(0).(*json.RawMessage)
	require.NotNil(t, stackTraces)
	var traces []string
	require.NoError(t, json.Unmarshal(*stackTraces, &traces))
	require.Equal(t, []string{stackTrace}, traces)

	var spanTags []map[string]any
	require.NoError(t, json.Unmarshal(fieldsByName["tags"].At(0).(json.RawMessage), &spanTags))
	require.Contains(t, spanTags, map[string]any{"key": "error", "value": true})

	client.AssertExpectations(t)
}

func TestQueryData_SingleTraceTable(t *testing.T) {
	to := time.Now()
	from := to.Add(-1 * time.Hour)
	tableFrameName := "traceTable"
	traceID := "123"
	startTime := timestamppb.New(time.UnixMilli(1660920349373))
	endTime := timestamppb.New(time.UnixMilli(1660920349374))

	spans := []*tracepb.TraceSpan{
		{
			SpanId:    1,
			Kind:      tracepb.TraceSpan_RPC_SERVER,
			Name:      "spanName",
			StartTime: startTime,
			EndTime:   endTime,
			Labels:    map[string]string{"key1": "value1"},
		},
	}
	trace := tracepb.Trace{
		ProjectId: "testProject",
		TraceId:   traceID,
		Spans:     spans,
	}

	client := mocks.NewAPI(t)
	client.On("ListTraces", mock.Anything, &cloudtrace.TracesQuery{
		ProjectID: "testing",
		Filter:    `resource.type:"testing"`,
		Limit:     20,
		TimeRange: cloudtrace.TimeRange{
			From: from,
			To:   to,
		},
	}).Return([]*tracepb.Trace{&trace}, nil)
	client.On("Close").Return(nil)

	ds := CloudTraceDatasource{
		client: client,
	}
	refID := "test"
	resp, err := ds.QueryData(context.Background(), &backend.QueryDataRequest{
		Queries: []backend.DataQuery{
			{
				JSON:  []byte(`{"projectId": "testing", "traceId": "123", "queryText": "resource.type:\"testing\""}`),
				RefID: refID,
				TimeRange: backend.TimeRange{
					From: from,
					To:   to,
				},
				MaxDataPoints: 20,
			},
		},
	})
	ds.Dispose()
	require.NoError(t, err)
	require.Len(t, resp.Responses[refID].Frames, 1)

	tableFrame := resp.Responses[refID].Frames[0]
	require.Equal(t, tableFrameName, tableFrame.Name)
	require.Len(t, tableFrame.Fields, 4)
	require.Equal(t, data.VisTypeTable, string(tableFrame.Meta.PreferredVisualization))

	expectedFrame := []byte(`{"schema":{"name":"traceTable","meta":{"typeVersion":[0,0],"preferredVisualisationType":"table"},"fields":[{"name":"Trace ID","type":"string","typeInfo":{"frame":"string"}},{"name":"Trace name","type":"string","typeInfo":{"frame":"string"}},{"name":"Start time","type":"time","typeInfo":{"frame":"time.Time","nullable":true}},{"name":"Latency","type":"number","typeInfo":{"frame":"int64","nullable":true},"config":{"unit":"ms"}}]},"data":{"values":[["123"],["spanName"],[1660920349373],[1]]}}`)

	serializedFrame, err := tableFrame.MarshalJSON()
	require.NoError(t, err)
	require.Equal(t, string(expectedFrame), string(serializedFrame))
	client.AssertExpectations(t)
}

func TestCreateTracesTableFrame_RootlessAndIncompleteTraces(t *testing.T) {
	startTime := time.UnixMilli(1660920349373).UTC()
	endTime := time.UnixMilli(1660920349380).UTC()

	traces := []*tracepb.Trace{
		{
			ProjectId: "testProject",
			TraceId:   "rootless-trace",
			Spans:     nil,
		},
		nil,
		{
			ProjectId: "testProject",
			TraceId:   "normal-trace",
			Spans: []*tracepb.TraceSpan{
				{
					SpanId:    1,
					Name:      "GET /health",
					StartTime: timestamppb.New(startTime),
					EndTime:   timestamppb.New(endTime),
				},
			},
		},
		{
			ProjectId: "testProject",
			TraceId:   "no-endtime-trace",
			Spans: []*tracepb.TraceSpan{
				{
					SpanId:    2,
					Name:      "in-flight",
					StartTime: timestamppb.New(startTime),
				},
			},
		},
	}

	frame := createTracesTableFrame(traces)

	rows, err := frame.RowLen()
	require.NoError(t, err)
	require.Equal(t, 3, rows)

	fieldsByName := map[string]*data.Field{}
	for _, f := range frame.Fields {
		fieldsByName[f.Name] = f
	}

	// Row 0: rootless trace keeps aligned columns and remains clickable
	require.Equal(t, "rootless-trace", fieldsByName["Trace ID"].At(0))
	require.Equal(t, "", fieldsByName["Trace name"].At(0))
	require.Nil(t, fieldsByName["Start time"].At(0))
	require.Nil(t, fieldsByName["Latency"].At(0))

	// Row 1: normal trace
	require.Equal(t, "normal-trace", fieldsByName["Trace ID"].At(1))
	require.Equal(t, "GET /health", fieldsByName["Trace name"].At(1))
	require.Equal(t, &startTime, fieldsByName["Start time"].At(1))
	expectedLatency := int64(7)
	require.Equal(t, &expectedLatency, fieldsByName["Latency"].At(1))

	// Row 2: root span missing EndTime
	require.Equal(t, "no-endtime-trace", fieldsByName["Trace ID"].At(2))
	require.Equal(t, "in-flight", fieldsByName["Trace name"].At(2))
	require.Equal(t, &startTime, fieldsByName["Start time"].At(2))
	require.Nil(t, fieldsByName["Latency"].At(2))
}

func TestSanitizeErrorMessage_PlainError(t *testing.T) {
	err := errors.New("connection refused")
	require.Equal(t, "connection refused", sanitizeErrorMessage(err))
}

func TestSanitizeErrorMessage_HTMLPage(t *testing.T) {
	err := errors.New(`<html><body><h1>502 Bad Gateway</h1></body></html>`)
	result := sanitizeErrorMessage(err)
	require.Contains(t, result, "HTML error page")
	require.NotContains(t, result, "<html>")
}

func TestSanitizeErrorMessage_DoctypeHTML(t *testing.T) {
	err := errors.New(`<!DOCTYPE html><html><body>Error</body></html>`)
	result := sanitizeErrorMessage(err)
	require.Contains(t, result, "HTML error page")
}

func TestSanitizeErrorMessage_ContentTypeHTML(t *testing.T) {
	err := errors.New(`received unexpected content-type: text/html`)
	result := sanitizeErrorMessage(err)
	require.Contains(t, result, "HTML error page")
}

func TestSanitizeErrorMessage_GoAngleBrackets(t *testing.T) {
	// Go errors containing <nil> should NOT be treated as HTML
	err := errors.New("x509: cannot parse <nil> as ASN.1")
	result := sanitizeErrorMessage(err)
	require.Equal(t, "x509: cannot parse <nil> as ASN.1", result)
}

// responseSender is a test helper that captures the CallResource response
type responseSender struct {
	resp *backend.CallResourceResponse
}

func (s *responseSender) Send(resp *backend.CallResourceResponse) error {
	s.resp = resp
	return nil
}

func TestCallResource_Projects(t *testing.T) {
	expectedProjects := []string{"project-a", "project-b", "project-c"}

	client := mocks.NewAPI(t)
	client.On("ListProjects", mock.Anything, "").Return(expectedProjects, nil)

	ds := &CloudTraceDatasource{
		client: client,
	}

	sender := &responseSender{}
	err := ds.CallResource(context.Background(), &backend.CallResourceRequest{
		Path: "projects",
		URL:  "projects",
	}, sender)

	require.NoError(t, err)
	require.NotNil(t, sender.resp)
	require.Equal(t, 200, sender.resp.Status)

	var projects []string
	err = json.Unmarshal(sender.resp.Body, &projects)
	require.NoError(t, err)
	require.Equal(t, expectedProjects, projects)
	client.AssertExpectations(t)
}

func TestCallResource_ProjectsWithQuery(t *testing.T) {
	expectedProjects := []string{"proj-a"}

	client := mocks.NewAPI(t)
	client.On("ListProjects", mock.Anything, "proj-a").Return(expectedProjects, nil)

	ds := &CloudTraceDatasource{
		client: client,
	}

	sender := &responseSender{}
	err := ds.CallResource(context.Background(), &backend.CallResourceRequest{
		Path: "projects",
		URL:  "projects?query=proj-a",
	}, sender)

	require.NoError(t, err)
	require.NotNil(t, sender.resp)
	require.Equal(t, 200, sender.resp.Status)

	var projects []string
	err = json.Unmarshal(sender.resp.Body, &projects)
	require.NoError(t, err)
	require.Equal(t, expectedProjects, projects)
	client.AssertExpectations(t)
}

func TestCallResource_ProjectsError(t *testing.T) {
	client := mocks.NewAPI(t)
	client.On("ListProjects", mock.Anything, "").Return(nil, errors.New("permission denied"))

	ds := &CloudTraceDatasource{
		client: client,
	}

	sender := &responseSender{}
	err := ds.CallResource(context.Background(), &backend.CallResourceRequest{
		Path: "projects",
		URL:  "projects",
	}, sender)

	require.NoError(t, err)
	require.NotNil(t, sender.resp)
	require.Equal(t, 502, sender.resp.Status)

	var body map[string]string
	err = json.Unmarshal(sender.resp.Body, &body)
	require.NoError(t, err)
	require.Equal(t, "permission denied", body["message"])
	client.AssertExpectations(t)
}

func TestCallResource_NotFound(t *testing.T) {
	client := mocks.NewAPI(t)

	ds := &CloudTraceDatasource{
		client: client,
	}

	sender := &responseSender{}
	err := ds.CallResource(context.Background(), &backend.CallResourceRequest{
		Path: "unknown",
		URL:  "unknown",
	}, sender)

	require.NoError(t, err)
	require.NotNil(t, sender.resp)
	require.Equal(t, 404, sender.resp.Status)

	var body map[string]string
	err = json.Unmarshal(sender.resp.Body, &body)
	require.NoError(t, err)
	require.Equal(t, "No such path", body["message"])
}


func newHealthRequest(jsonData string) *backend.CheckHealthRequest {
	return &backend.CheckHealthRequest{
		PluginContext: backend.PluginContext{
			DataSourceInstanceSettings: &backend.DataSourceInstanceSettings{JSONData: []byte(jsonData)},
		},
	}
}

func stubGCEDefaultProject(t *testing.T, fn func(context.Context, string) (string, error)) {
	t.Helper()
	orig := gceDefaultProject
	gceDefaultProject = fn
	t.Cleanup(func() { gceDefaultProject = orig })
}

func TestCheckHealth_GCEWithoutResolvableProject(t *testing.T) {
	// e.g. local `gcloud auth application-default login` user credentials carry no project.
	stubGCEDefaultProject(t, func(context.Context, string) (string, error) { return "", nil })
	client := mocks.NewAPI(t)
	ds := CloudTraceDatasource{client: client}

	res, err := ds.CheckHealth(context.Background(), newHealthRequest(`{"authenticationType":"gce"}`))

	require.NoError(t, err)
	require.Equal(t, backend.HealthStatusError, res.Status)
	require.Contains(t, res.Message, "Set Default project in the data source settings")
	client.AssertNotCalled(t, "TestConnection", mock.Anything, mock.Anything)
}

func TestCheckHealth_GCEUsesResolvedProject(t *testing.T) {
	stubGCEDefaultProject(t, func(context.Context, string) (string, error) { return "vm-project", nil })
	client := mocks.NewAPI(t)
	client.On("TestConnection", mock.Anything, "vm-project").Return(nil)
	ds := CloudTraceDatasource{client: client}

	res, err := ds.CheckHealth(context.Background(), newHealthRequest(`{"authenticationType":"gce"}`))

	require.NoError(t, err)
	require.Equal(t, backend.HealthStatusOk, res.Status)
	require.Equal(t, "Successfully queried traces from GCP project vm-project", res.Message)
}

func TestCheckHealth_GCELookupError(t *testing.T) {
	stubGCEDefaultProject(t, func(context.Context, string) (string, error) {
		return "", errors.New("could not find default credentials")
	})
	client := mocks.NewAPI(t)
	ds := CloudTraceDatasource{client: client}

	res, err := ds.CheckHealth(context.Background(), newHealthRequest(`{"authenticationType":"gce"}`))

	require.NoError(t, err)
	require.Equal(t, backend.HealthStatusError, res.Status)
	require.Equal(t, "failed to get GCE default project: could not find default credentials", res.Message)
}

func TestCheckHealth_ExplicitDefaultProjectSkipsGCELookup(t *testing.T) {
	stubGCEDefaultProject(t, func(context.Context, string) (string, error) {
		t.Fatal("the GCE default project lookup must not run when defaultProject is set")
		return "", nil
	})
	client := mocks.NewAPI(t)
	client.On("TestConnection", mock.Anything, "explicit-project").Return(nil)
	ds := CloudTraceDatasource{client: client}

	res, err := ds.CheckHealth(context.Background(), newHealthRequest(`{"authenticationType":"gce","defaultProject":"explicit-project"}`))

	require.NoError(t, err)
	require.Equal(t, backend.HealthStatusOk, res.Status)
}

func TestCheckHealth_AccessTokenWithoutProject(t *testing.T) {
	client := mocks.NewAPI(t)
	ds := CloudTraceDatasource{client: client}

	res, err := ds.CheckHealth(context.Background(), newHealthRequest(`{"authenticationType":"accessToken"}`))

	require.NoError(t, err)
	require.Equal(t, backend.HealthStatusError, res.Status)
	require.Equal(t, "No default project configured. Set the default project in the data source settings.", res.Message)
	client.AssertNotCalled(t, "TestConnection", mock.Anything, mock.Anything)
}

func TestCheckHealth_OAuthWithoutProjectKeepsMessage(t *testing.T) {
	client := mocks.NewAPI(t)
	ds := CloudTraceDatasource{client: client}

	res, err := ds.CheckHealth(context.Background(), newHealthRequest(`{"authenticationType":"oauthPassthrough","oauthPassThru":true}`))

	require.NoError(t, err)
	require.Equal(t, backend.HealthStatusError, res.Status)
	require.Equal(t, "Please define a default project for OAuth authentication", res.Message)
}
