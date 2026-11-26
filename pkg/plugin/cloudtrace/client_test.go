// Copyright 2026 Google LLC
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
	"context"
	"net"
	"testing"
	"time"

	trace "cloud.google.com/go/trace/apiv1"
	"cloud.google.com/go/trace/apiv1/tracepb"
	"github.com/stretchr/testify/require"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

type fakeTraceServer struct {
	tracepb.UnimplementedTraceServiceServer
	listTracesFn func(context.Context, *tracepb.ListTracesRequest) (*tracepb.ListTracesResponse, error)
}

func (f *fakeTraceServer) ListTraces(ctx context.Context, req *tracepb.ListTracesRequest) (*tracepb.ListTracesResponse, error) {
	return f.listTracesFn(ctx, req)
}

func newTestTraceClient(t *testing.T, srv tracepb.TraceServiceServer) *Client {
	t.Helper()

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	grpcServer := grpc.NewServer()
	tracepb.RegisterTraceServiceServer(grpcServer, srv)
	go func() {
		_ = grpcServer.Serve(lis)
	}()
	t.Cleanup(func() {
		grpcServer.Stop()
		_ = lis.Close()
	})

	conn, err := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = conn.Close()
	})

	tClient, err := trace.NewClient(context.Background(), option.WithGRPCConn(conn))
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = tClient.Close()
	})

	return &Client{tClient: tClient}
}

func TestListTraces_PropagatesFirstPageError(t *testing.T) {
	client := newTestTraceClient(t, &fakeTraceServer{
		listTracesFn: func(_ context.Context, _ *tracepb.ListTracesRequest) (*tracepb.ListTracesResponse, error) {
			return nil, status.Error(codes.PermissionDenied, "missing cloudtrace.traces.list permission")
		},
	})

	traces, err := client.ListTraces(context.Background(), &TracesQuery{
		ProjectID: "test-project",
		Limit:     10,
		TimeRange: TimeRange{From: time.Now().Add(-time.Hour), To: time.Now()},
	})

	require.Error(t, err)
	require.ErrorContains(t, err, "missing cloudtrace.traces.list permission")
	require.Nil(t, traces)
}

func TestListTraces_PropagatesSubsequentPageError(t *testing.T) {
	calls := 0
	client := newTestTraceClient(t, &fakeTraceServer{
		listTracesFn: func(_ context.Context, req *tracepb.ListTracesRequest) (*tracepb.ListTracesResponse, error) {
			calls++
			if req.PageToken == "" {
				return &tracepb.ListTracesResponse{
					Traces:        []*tracepb.Trace{{TraceId: "trace-1"}},
					NextPageToken: "page-2",
				}, nil
			}
			return nil, status.Error(codes.Internal, "backend page failure")
		},
	})

	traces, err := client.ListTraces(context.Background(), &TracesQuery{
		ProjectID: "test-project",
		Limit:     10,
		TimeRange: TimeRange{From: time.Now().Add(-time.Hour), To: time.Now()},
	})

	require.Equal(t, 2, calls)
	require.Error(t, err)
	require.ErrorContains(t, err, "backend page failure")
	require.Nil(t, traces)
}

func TestListTraces_DefaultLimitWhenZero(t *testing.T) {
	var seenPageSize int32
	client := newTestTraceClient(t, &fakeTraceServer{
		listTracesFn: func(_ context.Context, req *tracepb.ListTracesRequest) (*tracepb.ListTracesResponse, error) {
			seenPageSize = req.PageSize
			return &tracepb.ListTracesResponse{
				Traces: []*tracepb.Trace{
					{TraceId: "trace-1"},
					{TraceId: "trace-2"},
					{TraceId: "trace-3"},
				},
			}, nil
		},
	})

	traces, err := client.ListTraces(context.Background(), &TracesQuery{
		ProjectID: "test-project",
		Limit:     0,
		TimeRange: TimeRange{From: time.Now().Add(-time.Hour), To: time.Now()},
	})

	require.NoError(t, err)
	require.Equal(t, int32(defaultListTracesLimit), seenPageSize)
	require.Len(t, traces, 3)
}
