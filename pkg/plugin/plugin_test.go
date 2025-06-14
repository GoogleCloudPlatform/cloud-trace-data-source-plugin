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
	require.Len(t, traceFrame.Fields, 9)
	require.Equal(t, data.VisTypeTrace, string(traceFrame.Meta.PreferredVisualization))

	expectedFrame := []byte(`{"schema":{"name":"123","meta":{"typeVersion":[0,0],"preferredVisualisationType":"trace"},"fields":[{"name":"traceID","type":"string","typeInfo":{"frame":"string"}},{"name":"parentSpanID","type":"string","typeInfo":{"frame":"string"}},{"name":"spanID","type":"string","typeInfo":{"frame":"string"}},{"name":"serviceName","type":"string","typeInfo":{"frame":"string"}},{"name":"operationName","type":"string","typeInfo":{"frame":"string"}},{"name":"serviceTags","type":"other","typeInfo":{"frame":"json.RawMessage"}},{"name":"tags","type":"other","typeInfo":{"frame":"json.RawMessage"}},{"name":"startTime","type":"time","typeInfo":{"frame":"time.Time"}},{"name":"duration","type":"number","typeInfo":{"frame":"float64"}}]},"data":{"values":[["123"],["0"],["1"],[""],["spanName"],[[]],[[{"key":"key1","value":"value1"}]],[1660920349373],[1]]}}`)

	serializedFrame, err := traceFrame.MarshalJSON()
	require.NoError(t, err)
	require.Equal(t, string(expectedFrame), string(serializedFrame))

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

	expectedFrame := []byte(`{"schema":{"name":"traceTable","meta":{"typeVersion":[0,0],"preferredVisualisationType":"table"},"fields":[{"name":"Trace ID","type":"string","typeInfo":{"frame":"string"}},{"name":"Trace name","type":"string","typeInfo":{"frame":"string"}},{"name":"Start time","type":"time","typeInfo":{"frame":"time.Time"}},{"name":"Latency","type":"number","typeInfo":{"frame":"int64"},"config":{"unit":"ms"}}]},"data":{"values":[["123"],["spanName"],[1660920349373],[1]]}}`)

	serializedFrame, err := tableFrame.MarshalJSON()
	require.NoError(t, err)
	require.Equal(t, string(expectedFrame), string(serializedFrame))
	client.AssertExpectations(t)
}
