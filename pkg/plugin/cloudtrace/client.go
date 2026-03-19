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
	"strings"
	"time"

	trace "cloud.google.com/go/trace/apiv1"
	"cloud.google.com/go/trace/apiv1/tracepb"
	resourcemanager "cloud.google.com/go/resourcemanager/apiv3"
	resourcemanagerpb "cloud.google.com/go/resourcemanager/apiv3/resourcemanagerpb"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"golang.org/x/oauth2"
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
	// ListProjects returns the project IDs of all visible projects.
	// If query is non-empty it is forwarded to the Resource Manager search filter.
	ListProjects(ctx context.Context, query string) ([]string, error)
	// Close closes the underlying connection to the GCP API
	Close() error
}

// Client wraps a GCP trace client to fetch traces and spans,
// and a resourcemanager client to list projects
type Client struct {
	tClient *trace.Client
	rClient *resourcemanager.ProjectsClient
}

func universeDomainOpts(universeDomain string) []option.ClientOption {
	if universeDomain == "" {
		return nil
	}
	return []option.ClientOption{option.WithUniverseDomain(universeDomain)}
}

// NewClient creates a new Client using jsonCreds for authentication
func NewClient(ctx context.Context, jsonCreds []byte, universeDomain string) (*Client, error) {
	opts := append([]option.ClientOption{
		option.WithCredentialsJSON(jsonCreds),
		option.WithUserAgent("googlecloud-trace-datasource"),
	}, universeDomainOpts(universeDomain)...)

	client, err := trace.NewClient(ctx, opts...)
	if err != nil {
		return nil, err
	}
	rClient, err := resourcemanager.NewProjectsClient(ctx, opts...)
	if err != nil {
		_ = client.Close()
		return nil, err
	}

	return &Client{
		tClient: client,
		rClient: rClient,
	}, nil
}

// NewClientWithGCE creates a new Client using GCE metadata for authentication
func NewClientWithGCE(ctx context.Context, universeDomain string) (*Client, error) {
	opts := append([]option.ClientOption{
		option.WithUserAgent("googlecloud-trace-datasource"),
	}, universeDomainOpts(universeDomain)...)

	client, err := trace.NewClient(ctx, opts...)
	if err != nil {
		return nil, err
	}
	rClient, err := resourcemanager.NewProjectsClient(ctx, opts...)
	if err != nil {
		_ = client.Close()
		return nil, err
	}

	return &Client{
		tClient: client,
		rClient: rClient,
	}, nil
}

// NewClientWithImpersonation creates a new Client using service account impersonation
func NewClientWithImpersonation(ctx context.Context, jsonCreds []byte, impersonateSA string, universeDomain string) (*Client, error) {
	var ts oauth2.TokenSource
	var err error

	impersonateOpts := append([]option.ClientOption{}, universeDomainOpts(universeDomain)...)

	if jsonCreds == nil {
		ts, err = impersonate.CredentialsTokenSource(ctx, impersonate.CredentialsConfig{
			TargetPrincipal: impersonateSA,
			Scopes:          []string{"https://www.googleapis.com/auth/cloud-platform"},
		}, impersonateOpts...)
	} else {
		impersonateOpts = append(impersonateOpts, option.WithCredentialsJSON(jsonCreds))
		ts, err = impersonate.CredentialsTokenSource(ctx, impersonate.CredentialsConfig{
			TargetPrincipal: impersonateSA,
			Scopes:          []string{"https://www.googleapis.com/auth/cloud-platform"},
		}, impersonateOpts...)
	}
	if err != nil {
		return nil, err
	}

	opts := append([]option.ClientOption{
		option.WithTokenSource(ts),
		option.WithUserAgent("googlecloud-trace-datasource"),
	}, universeDomainOpts(universeDomain)...)

	client, err := trace.NewClient(ctx, opts...)
	if err != nil {
		return nil, err
	}
	rClient, err := resourcemanager.NewProjectsClient(ctx, opts...)
	if err != nil {
		_ = client.Close()
		return nil, err
	}

	return &Client{
		tClient: client,
		rClient: rClient,
	}, nil
}

// NewClientWithAccessToken creates a new Client using an access token for authentication.
// Since the datasource is re-created whenever the token changes, we can treat this token as static.
func NewClientWithAccessToken(ctx context.Context, accessToken string, universeDomain string) (*Client, error) {
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: accessToken})

	opts := append([]option.ClientOption{
		option.WithTokenSource(ts),
		option.WithUserAgent("googlecloud-trace-datasource"),
	}, universeDomainOpts(universeDomain)...)

	client, err := trace.NewClient(ctx, opts...)
	if err != nil {
		return nil, err
	}

	rClient, err := resourcemanager.NewProjectsClient(ctx, opts...)
	if err != nil {
		_ = client.Close()
		return nil, err
	}

	return &Client{
		tClient: client,
		rClient: rClient,
	}, nil
}

// NewClientWithPassThrough creates a new Client using OAuth browser credentials
func NewClientWithPassThrough(ctx context.Context, headers map[string]string, universeDomain string) (*Client, error) {
	token, found := strings.CutPrefix(headers["Authorization"], "Bearer ")
	if !found || token == "" {
		return nil, errors.New("missing or invalid Authorization header")
	}

	opts := append([]option.ClientOption{
		option.WithTokenSource(
			oauth2.StaticTokenSource(&oauth2.Token{
				AccessToken: token,
			}),
		),
		option.WithUserAgent("googlecloud-trace-datasource"),
	}, universeDomainOpts(universeDomain)...)

	client, err := trace.NewClient(ctx, opts...)
	if err != nil {
		return nil, err
	}
	rClient, err := resourcemanager.NewProjectsClient(ctx, opts...)
	if err != nil {
		_ = client.Close()
		return nil, err
	}

	return &Client{
		tClient: client,
		rClient: rClient,
	}, nil
}

// Close closes the underlying connection to the GCP API
func (c *Client) Close() error {
	c.rClient.Close()
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

// ListProjects returns the project IDs of all visible projects.
// If query is non-empty it is forwarded to the Resource Manager
// SearchProjects API which supports free-text search (AIP-160).
// Results are capped at maxProjects.
const maxProjects = 100

func (c *Client) ListProjects(ctx context.Context, query string) ([]string, error) {
	filter := ""
	if query != "" {
		filter = fmt.Sprintf("id:*%s* OR name:*%s*", query, query)
	}

	projectIDs := []string{}
	req := &resourcemanagerpb.SearchProjectsRequest{
		Query:    filter,
		PageSize: maxProjects,
	}
	it := c.rClient.SearchProjects(ctx, req)
	for {
		project, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		if project.State != resourcemanagerpb.Project_ACTIVE {
			continue
		}
		projectIDs = append(projectIDs, project.ProjectId)
		if len(projectIDs) >= maxProjects {
			break
		}
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
