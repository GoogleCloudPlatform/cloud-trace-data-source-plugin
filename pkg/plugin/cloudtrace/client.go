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
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	trace "cloud.google.com/go/trace/apiv1"
	"cloud.google.com/go/trace/apiv1/tracepb"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"golang.org/x/oauth2"
	resourcemanager "google.golang.org/api/cloudresourcemanager/v1"
	"google.golang.org/api/impersonate"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
	"google.golang.org/protobuf/types/known/timestamppb"

	cloudtracepb "cloud.google.com/go/trace/apiv1/tracepb"
)

const testConnectionTimeWindow = time.Hour * 24 * 30 // 30 days

// API implements the methods we need to query traces and list projects from GCP
type API interface {
	// ListTraces retrieves all traces matching some query filter up to the given limit
	ListTraces(context.Context, *TracesQuery) ([]*cloudtracepb.Trace, error)
	// GetTrace retrieves a trace matching a trace ID
	GetTrace(context.Context, *TraceQuery) (*cloudtracepb.Trace, error)
	// TestConnection queries for any trace from the given project
	TestConnection(ctx context.Context, projectID string) error
	// ListProjects returns the project IDs of all visible projects
	ListProjects(context.Context) ([]string, error)
	// Close closes the underlying connection to the GCP API
	Close() error
}

// Client wraps a GCP trace client to fetch traces and spance,
// and a resourcemanager client to list projects
type Client struct {
	tClient *trace.Client
	rClient *resourcemanager.ProjectsService
}

// NewClient creates a new Client using jsonCreds for authentication
func NewClient(ctx context.Context, jsonCreds []byte) (*Client, error) {
	client, err := trace.NewClient(ctx, option.WithCredentialsJSON(jsonCreds),
		option.WithUserAgent("googlecloud-trace-datasource"))
	if err != nil {
		return nil, err
	}
	rClient, err := resourcemanager.NewService(ctx, option.WithCredentialsJSON(jsonCreds),
		option.WithUserAgent("googlecloud-trace-datasource"))
	if err != nil {
		return nil, err
	}

	return &Client{
		tClient: client,
		rClient: rClient.Projects,
	}, nil
}

// NewClient creates a new Client using GCE metadata for authentication
func NewClientWithGCE(ctx context.Context) (*Client, error) {
	client, err := trace.NewClient(ctx,
		option.WithUserAgent("googlecloud-trace-datasource"))
	if err != nil {
		return nil, err
	}
	rClient, err := resourcemanager.NewService(ctx,
		option.WithUserAgent("googlecloud-trace-datasource"))
	if err != nil {
		return nil, err
	}

	return &Client{
		tClient: client,
		rClient: rClient.Projects,
	}, nil
}

// NewClient creates a new Clients using service account impersonation
func NewClientWithImpersonation(ctx context.Context, jsonCreds []byte, impersonateSA string) (*Client, error) {
	var ts oauth2.TokenSource
	var err error
	if jsonCreds == nil {
		ts, err = impersonate.CredentialsTokenSource(ctx, impersonate.CredentialsConfig{
			TargetPrincipal: impersonateSA,
			Scopes:          []string{"https://www.googleapis.com/auth/cloud-platform"},
		})
	} else {
		ts, err = impersonate.CredentialsTokenSource(ctx, impersonate.CredentialsConfig{
			TargetPrincipal: impersonateSA,
			Scopes:          []string{"https://www.googleapis.com/auth/cloud-platform"},
		}, option.WithCredentialsJSON(jsonCreds))
	}
	if err != nil {
		return nil, err
	}

	client, err := trace.NewClient(ctx, option.WithTokenSource(ts),
		option.WithUserAgent("googlecloud-trace-datasource"))
	if err != nil {
		return nil, err
	}
	rClient, err := resourcemanager.NewService(ctx, option.WithTokenSource(ts),
		option.WithUserAgent("googlecloud-trace-datasource"))
	if err != nil {
		return nil, err
	}

	return &Client{
		tClient: client,
		rClient: rClient.Projects,
	}, nil
}

// Close closes the underlying connection to the GCP API
func (c *Client) Close() error {
	return c.tClient.Close()
}

// TracesQuery is the information from a Grafana query needed to query GCP for traces
type TracesQuery struct {
	ProjectID string
	Filter    string
	Limit     int64
	TimeRange TimeRange
}

// TraceQuery is the information from a Grafana query needed to query GCP for a trace
type TraceQuery struct {
	ProjectID string
	TraceID   string
}

// ListProjects returns the project IDs of all visible projects
func (c *Client) ListProjects(ctx context.Context) ([]string, error) {
	response, err := c.rClient.List().Do()
	if err != nil {
		return nil, err
	}

	projectIDs := []string{}
	for _, p := range response.Projects {
		if p.LifecycleState == "DELETE_REQUESTED" || p.LifecycleState == "DELETE_IN_PROGRESS" {
			continue
		}
		projectIDs = append(projectIDs, p.ProjectId)
	}
	return projectIDs, nil
}

// TestConnection queries for any trace from the given project
func (c *Client) TestConnection(ctx context.Context, projectID string) error {
	start := time.Now()

	listCtx, cancel := context.WithTimeout(ctx, time.Duration(time.Minute*1))

	defer func() {
		cancel()
		log.DefaultLogger.Info("Finished testConnection", "duration", time.Since(start).String())
	}()

	it := c.tClient.ListTraces(listCtx, &cloudtracepb.ListTracesRequest{
		ProjectId: projectID,
		PageSize:  1,
		StartTime: timestamppb.New(time.Now().Add(-testConnectionTimeWindow)),
	})

	if listCtx.Err() != nil {
		return errors.New("timeout")
	}

	entry, err := it.Next()
	if err == iterator.Done {
		return errors.New("no entries")
	}
	if err == context.DeadlineExceeded {
		return errors.New("list entries: timeout")
	}
	if err != nil {
		return fmt.Errorf("list entries: %w", err)
	}
	if entry == nil {
		return errors.New("no entries")
	}

	return nil
}

// ListTraces retrieves all traces matching some query filter up to the given limit
func (c *Client) ListTraces(ctx context.Context, q *TracesQuery) ([]*cloudtracepb.Trace, error) {
	// Never exceed the maximum page size
	pageSize := int32(math.Min(float64(q.Limit), 1000))

	req := cloudtracepb.ListTracesRequest{
		ProjectId: q.ProjectID,
		Filter:    q.Filter,
		StartTime: timestamppb.New(q.TimeRange.From),
		EndTime:   timestamppb.New(q.TimeRange.To),
		OrderBy:   "start desc",
		PageSize:  pageSize,
		View:      tracepb.ListTracesRequest_ROOTSPAN,
	}

	start := time.Now()
	defer func() {
		log.DefaultLogger.Info("Finished listing traces", "duration", time.Since(start).String())
	}()

	it := c.tClient.ListTraces(ctx, &req)
	if it == nil {
		return nil, errors.New("nil response")
	}

	var i int64
	entries := []*cloudtracepb.Trace{}
	for {
		resp, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			log.DefaultLogger.Error("error getting page", "error", err)
			break
		}

		entries = append(entries, resp)
		i++
		if i >= q.Limit {
			break
		}
	}
	return entries, nil
}

// GetTrace retrieves a single trace given a trace ID
func (c *Client) GetTrace(ctx context.Context, q *TraceQuery) (*cloudtracepb.Trace, error) {
	req := cloudtracepb.GetTraceRequest{
		ProjectId: q.ProjectID,
		TraceId:   q.TraceID,
	}

	start := time.Now()
	defer func() {
		log.DefaultLogger.Info(fmt.Sprintf("Finished getting trace: %s", q.TraceID), "duration", time.Since(start).String())
	}()

	trace, err := c.tClient.GetTrace(ctx, &req)
	if err != nil {
		return nil, err
	}
	if trace == nil {
		return nil, errors.New("nil response")
	}

	return trace, nil
}
