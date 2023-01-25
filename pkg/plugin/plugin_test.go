package plugin

import (
	"context"
	"errors"
	"testing"
	"time"

	"cloud.google.com/go/trace/apiv1/tracepb"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data"
	cloudtrace "github.com/observiq/cloud-trace-grafana-ds/pkg/plugin/cloudtrace"
	"github.com/observiq/cloud-trace-grafana-ds/pkg/plugin/mocks"
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

func TestQueryData_GCPError(t *testing.T) {
	to := time.Now()
	from := to.Add(-1 * time.Hour)
	expectedErr := errors.New("something was wrong with the request")

	client := mocks.NewAPI(t)
	client.On("ListTraces", mock.Anything, &cloudtrace.Query{
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

func TestQueryData_SingleTrace(t *testing.T) {
	to := time.Now()
	from := to.Add(-1 * time.Hour)
	frameName := "traceTable"
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
		TraceId:   "traceID",
		Spans:     spans,
	}

	client := mocks.NewAPI(t)
	client.On("ListTraces", mock.Anything, &cloudtrace.Query{
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
	ds.Dispose()
	require.NoError(t, err)
	require.Len(t, resp.Responses[refID].Frames, 1)

	frame := resp.Responses[refID].Frames[0]
	require.Equal(t, frameName, frame.Name)
	require.Len(t, frame.Fields, 4)
	require.Equal(t, data.VisTypeTable, string(frame.Meta.PreferredVisualization))

	expectedFrame := []byte(`{"schema":{"name":"traceTable","meta":{"preferredVisualisationType":"table"},"fields":[{"name":"Trace ID","type":"string","typeInfo":{"frame":"string"}},{"name":"Trace name","type":"string","typeInfo":{"frame":"string"}},{"name":"Start time","type":"time","typeInfo":{"frame":"time.Time"}},{"name":"Latency","type":"number","typeInfo":{"frame":"int64"},"config":{"unit":"ms"}}]},"data":{"values":[["traceID"],["spanName"],[1660920349373],[1]]}}`)

	serializedFrame, err := frame.MarshalJSON()
	require.NoError(t, err)
	require.Equal(t, string(expectedFrame), string(serializedFrame))
	client.AssertExpectations(t)
}
