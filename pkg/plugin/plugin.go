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
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"cloud.google.com/go/trace/apiv1/tracepb"
	cloudtrace "github.com/GoogleCloudPlatform/cloud-trace-data-source-plugin/pkg/plugin/cloudtrace"
	"github.com/grafana/grafana-google-sdk-go/pkg/utils"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/instancemgmt"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/grafana/grafana-plugin-sdk-go/data"
)

// Make sure CloudTraceDatasource implements required interfaces
var (
	_                     backend.QueryDataHandler      = (*CloudTraceDatasource)(nil)
	_                     backend.CheckHealthHandler    = (*CloudTraceDatasource)(nil)
	_                     instancemgmt.InstanceDisposer = (*CloudTraceDatasource)(nil)
	errMissingCredentials                               = errors.New("missing credentials")
	errMissingAccessToken                               = errors.New("missing access token")
)

const (
	privateKeyKey                = "privateKey"
	gceAuthentication            = "gce"
	jwtAuthentication            = "jwt"
	accessTokenAuthentication    = "accessToken"
	accessTokenKey               = "accessToken"
	oauthPassThruAuthentication  = "oauthPassthrough"
)

// config is the fields parsed from the front end
type config struct {
	AuthType                    string `json:"authenticationType"`
	ClientEmail                 string `json:"clientEmail"`
	DefaultProject              string `json:"defaultProject"`
	TokenURI                    string `json:"tokenUri"`
	ServiceAccountToImpersonate string `json:"serviceAccountToImpersonate"`
	UsingImpersonation          bool   `json:"usingImpersonation"`
	OAuthPassThru               bool   `json:"oauthPassThru"`
	UniverseDomain              string `json:"universeDomain"`
}

// toServiceAccountJSON creates the serviceAccountJSON bytes from the config fields
func (c config) toServiceAccountJSON(privateKey string) ([]byte, error) {
	return json.Marshal(serviceAccountJSON{
		Type:        "service_account",
		ProjectID:   c.DefaultProject,
		PrivateKey:  privateKey,
		ClientEmail: c.ClientEmail,
		TokenURI:    c.TokenURI,
	})
}

// serviceAccountJSON is the expected structure of a GCP Service Account credentials file
// We mainly want to be able to pull out ProjectID to use as a default
type serviceAccountJSON struct {
	Type        string `json:"type"`
	ProjectID   string `json:"project_id"`
	PrivateKey  string `json:"private_key"`
	ClientEmail string `json:"client_email"`
	TokenURI    string `json:"token_uri"`
}

// NewCloudTraceDatasource creates a new datasource instance.
func NewCloudTraceDatasource(ctx context.Context, settings backend.DataSourceInstanceSettings) (instancemgmt.Instance, error) {
	var conf config
	if err := json.Unmarshal(settings.JSONData, &conf); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}

	if conf.AuthType == "" {
		conf.AuthType = jwtAuthentication
	}

	// Check if access token is configured and switch auth type if present
	if accessToken, ok := settings.DecryptedSecureJSONData[accessTokenKey]; ok && accessToken != "" {
		conf.AuthType = accessTokenAuthentication
	}

	// For OAuth passthrough, we don't create a persistent client.
	// Instead, a per-request client is created using the forwarded token.
	if conf.AuthType == oauthPassThruAuthentication || conf.OAuthPassThru {
		return &CloudTraceDatasource{
			oauthPassThrough: true,
			universeDomain:   conf.UniverseDomain,
		}, nil
	}

	var client_err error
	var client *cloudtrace.Client

	switch conf.AuthType {
	case jwtAuthentication:
		privateKey, ok := settings.DecryptedSecureJSONData[privateKeyKey]
		if !ok || privateKey == "" {
			return nil, errMissingCredentials
		}

		serviceAccount, err := conf.toServiceAccountJSON(privateKey)
		if err != nil {
			return nil, fmt.Errorf("create credentials: %w", err)
		}
		if conf.UsingImpersonation {
			client, client_err = cloudtrace.NewClientWithImpersonation(context.TODO(), serviceAccount, conf.ServiceAccountToImpersonate, conf.UniverseDomain)
		} else {
			client, client_err = cloudtrace.NewClient(context.TODO(), serviceAccount, conf.UniverseDomain)
		}
	case gceAuthentication:
		if conf.UsingImpersonation {
			client, client_err = cloudtrace.NewClientWithImpersonation(context.TODO(), nil, conf.ServiceAccountToImpersonate, conf.UniverseDomain)
		} else {
			client, client_err = cloudtrace.NewClientWithGCE(context.TODO(), conf.UniverseDomain)
		}
	case accessTokenAuthentication:
		accessToken, ok := settings.DecryptedSecureJSONData[accessTokenKey]
		if !ok || accessToken == "" {
			return nil, errMissingAccessToken
		}
		client, client_err = cloudtrace.NewClientWithAccessToken(context.TODO(), accessToken, conf.UniverseDomain)
	default:
		return nil, fmt.Errorf("unknown authentication type: %s", conf.AuthType)
	}

	if client_err != nil {
		return nil, fmt.Errorf("create client: %w", client_err)
	}

	return &CloudTraceDatasource{
		client:         client,
		universeDomain: conf.UniverseDomain,
	}, nil
}

// CloudTraceDatasource is an example datasource which can respond to data queries, reports
// its health and has streaming skills.
type CloudTraceDatasource struct {
	client           cloudtrace.API
	oauthPassThrough bool
	universeDomain   string
}

// Dispose here tells plugin SDK that plugin wants to clean up resources when a new instance
// created. As soon as datasource settings change detected by SDK old datasource instance will
// be disposed and a new one will be created using NewSampleDatasource factory function.
func (d *CloudTraceDatasource) Dispose() {
	if d.client != nil {
		if err := d.client.Close(); err != nil {
			log.DefaultLogger.Error("failed closing client", "error", err)
		}
	}
}

// jsonErrorBody returns a JSON-encoded body with a "message" field so that
// Grafana's frontend can surface the actual error text instead of the generic
// "Unexpected error" fallback.
func jsonErrorBody(msg string) []byte {
	b, _ := json.Marshal(map[string]string{"message": msg})
	return b
}

// CallResource fetches some resource from GCP using the data source's credentials
//
// Currently only projects are fetched, other requests receive a 404
func (d *CloudTraceDatasource) CallResource(ctx context.Context, req *backend.CallResourceRequest, sender backend.CallResourceResponseSender) error {
	// log.DefaultLogger.Info("CallResource called")

	client := d.client

	if d.oauthPassThrough {
		headers := make(map[string]string)
		for k, v := range req.Headers {
			if strings.EqualFold(k, "Authorization") {
				headers["Authorization"] = v[0]
				break
			}
		}
		oauthClient, err := d.CreateOauthClient(ctx, headers)
		if err != nil {
			return sender.Send(&backend.CallResourceResponse{
				Status: http.StatusBadGateway,
				Body:   jsonErrorBody(sanitizeErrorMessage(err)),
			})
		}
		client = oauthClient
		defer client.Close()
	}

	var body []byte

	// Right now we only support calls to `gceDefaultProject` and `/projects`
	resource := req.Path

	if resource == "gceDefaultProject" {
		proj, err := utils.GCEDefaultProject(ctx, "")
		if err != nil {
			log.DefaultLogger.Error("problem getting GCE default project", "error", err)
			return sender.Send(&backend.CallResourceResponse{
				Status: http.StatusBadGateway,
				Body:   jsonErrorBody(sanitizeErrorMessage(err)),
			})
		}
		body, err = json.Marshal(proj)
		if err != nil {
			return sender.Send(&backend.CallResourceResponse{
				Status: http.StatusInternalServerError,
				Body:   jsonErrorBody("Unable to create response"),
			})
		}
	} else if strings.ToLower(resource) != "projects" {
		return sender.Send(&backend.CallResourceResponse{
			Status: http.StatusNotFound,
			Body:   jsonErrorBody("No such path"),
		})
	} else {
		reqUrl, err := url.Parse(req.URL)
		if err != nil {
			return sender.Send(&backend.CallResourceResponse{
				Status: http.StatusBadRequest,
				Body:   jsonErrorBody("Invalid request URL"),
			})
		}
		searchQuery := reqUrl.Query().Get("query")

		projects, err := client.ListProjects(ctx, searchQuery)
		if err != nil {
			log.DefaultLogger.Error("problem listing projects", "error", err)
			return sender.Send(&backend.CallResourceResponse{
				Status: http.StatusBadGateway,
				Body:   jsonErrorBody(sanitizeErrorMessage(err)),
			})
		}

		body, err = json.Marshal(projects)
		if err != nil {
			return sender.Send(&backend.CallResourceResponse{
				Status: http.StatusInternalServerError,
				Body:   jsonErrorBody("Unable to create response"),
			})
		}
	}

	return sender.Send(&backend.CallResourceResponse{
		Status: http.StatusOK,
		Body:   body,
	})
}

// QueryData handles multiple queries and returns multiple responses.
// req contains the queries []DataQuery (where each query contains RefID as a unique identifier).
// The QueryDataResponse contains a map of RefID to the response for each query, and each response
// contains Frames ([]*Frame).
func (d *CloudTraceDatasource) QueryData(ctx context.Context, req *backend.QueryDataRequest) (*backend.QueryDataResponse, error) {
	// log.DefaultLogger.Info("QueryData called")
	client := d.client

	if d.oauthPassThrough {
		oauthClient, err := d.CreateOauthClient(ctx, req.Headers)
		if err != nil {
			response := backend.NewQueryDataResponse()
			for _, q := range req.Queries {
				response.Responses[q.RefID] = backend.DataResponse{
					Error: fmt.Errorf("%s", sanitizeErrorMessage(err)),
				}
			}
			return response, nil
		}
		client = oauthClient
		defer client.Close()
	}

	// create response struct
	response := backend.NewQueryDataResponse()

	// loop over queries and execute them individually.
	for _, q := range req.Queries {
		res := d.query(ctx, req.PluginContext, q, client)

		// save the response in a hashmap
		// based on with RefID as identifier
		response.Responses[q.RefID] = res
	}

	return response, nil
}

// queryModel is the fields needed to query from Grafana
type queryModel struct {
	TraceID       string `json:"traceId"`
	QueryText     string `json:"queryText"`
	QueryType     string `json:"queryType"`
	ProjectID     string `json:"projectId"`
	MaxDataPoints int    `json:"MaxDataPoints"`
}

func (d *CloudTraceDatasource) query(ctx context.Context, pCtx backend.PluginContext, query backend.DataQuery, client cloudtrace.API) backend.DataResponse {
	response := backend.DataResponse{}

	var q queryModel
	response.Error = json.Unmarshal(query.JSON, &q)
	if response.Error != nil {
		return response
	}

	if q.QueryType == "traceID" && strings.TrimSpace(q.TraceID) != "" {
		f, err := d.getTraceSpanFrame(ctx, q, client)
		if err != nil {
			response.Error = fmt.Errorf("trace query: %s", sanitizeErrorMessage(err))
			return response
		}

		response.Frames = append(response.Frames, f)
	}

	if q.QueryType == "" {
		f, err := d.getTracesTableFrame(ctx, q, query, client)
		if err != nil {
			response.Error = fmt.Errorf("filter query: %s", sanitizeErrorMessage(err))
			return response
		}

		response.Frames = append(response.Frames, f)
	}

	return response
}

func (d *CloudTraceDatasource) getTraceSpanFrame(ctx context.Context, q queryModel, client cloudtrace.API) (*data.Frame, error) {
	clientRequest := cloudtrace.TraceQuery{
		ProjectID: q.ProjectID,
		TraceID:   q.TraceID,
	}

	trace, err := client.GetTrace(ctx, &clientRequest)
	if err != nil {
		return nil, err
	}

	f := createTraceSpanFrame(trace)

	return f, nil
}

func createTraceSpanFrame(trace *tracepb.Trace) *data.Frame {
	// Create one frame for all trace/spans
	f := data.NewFrame(trace.GetTraceId())
	f.Meta = &data.FrameMeta{}
	f.Meta.PreferredVisualization = data.VisTypeTrace

	// Create one set of fields for all trace/spans
	traceIDField := data.NewField("traceID", nil, []string{})
	spanIDField := data.NewField("spanID", nil, []string{})
	parentSpanIDField := data.NewField("parentSpanID", nil, []string{})
	operationNameField := data.NewField("operationName", nil, []string{})
	serviceNameField := data.NewField("serviceName", nil, []string{})
	serviceTagsField := data.NewField("serviceTags", nil, []json.RawMessage{})
	startTimeField := data.NewField("startTime", nil, []time.Time{})
	durationField := data.NewField("duration", nil, []float64{})
	tagsField := data.NewField("tags", nil, []json.RawMessage{})

	// Add values to each field for each span
	for _, s := range trace.Spans {
		serviceTags, spanTags, err := cloudtrace.GetTags(s)
		if err != nil {
			log.DefaultLogger.Warn("failed getting span tags", "error", err)
			continue
		}
		tagsField.Append(spanTags)
		serviceTagsField.Append(serviceTags)

		traceIDField.Append(trace.GetTraceId())
		spanIDField.Append(strconv.FormatUint(s.GetSpanId(), 10))
		parentSpanIDField.Append(strconv.FormatUint(s.GetParentSpanId(), 10))
		operationNameField.Append(cloudtrace.GetSpanOperationName(s))
		serviceNameField.Append(cloudtrace.GetServiceName(s))
		startTimeField.Append(s.GetStartTime().AsTime())
		duration := float64(s.GetEndTime().AsTime().UnixMicro()-s.GetStartTime().AsTime().UnixMicro()) / 1000
		durationField.Append(duration)
	}

	f.Fields = append(f.Fields,
		traceIDField,
		parentSpanIDField,
		spanIDField,
		serviceNameField,
		operationNameField,
		serviceTagsField,
		tagsField,
		startTimeField,
		durationField,
	)

	return f
}

func (d *CloudTraceDatasource) getTracesTableFrame(ctx context.Context, q queryModel, dQuery backend.DataQuery, client cloudtrace.API) (*data.Frame, error) {
	filter, err := cloudtrace.GetListTracesFilter(q.QueryText)
	if err != nil {
		return nil, err
	}

	clientRequest := cloudtrace.TracesQuery{
		ProjectID: q.ProjectID,
		Filter:    filter,
		Limit:     dQuery.MaxDataPoints,
		TimeRange: cloudtrace.TimeRange{
			From: dQuery.TimeRange.From,
			To:   dQuery.TimeRange.To,
		},
	}

	traces, err := client.ListTraces(ctx, &clientRequest)
	if err != nil {
		return nil, err
	}

	f := createTracesTableFrame(traces)

	return f, nil
}

func createTracesTableFrame(traces []*tracepb.Trace) *data.Frame {
	// Create one frame for all traces
	f := data.NewFrame("traceTable")
	f.Meta = &data.FrameMeta{}
	f.Meta.PreferredVisualization = data.VisTypeTable

	// Create one set of fields for all traces
	tableTraceIDField := data.NewField("Trace ID", nil, []string{})
	tableTraceNameField := data.NewField("Trace name", nil, []string{})
	tableStartTimeField := data.NewField("Start time", nil, []time.Time{})
	tableLatencyField := data.NewField("Latency", nil, []int64{})
	tableLatencyField.Config = &data.FieldConfig{
		Unit: "ms",
	}

	// Add values to each field for each trace
	for _, t := range traces {
		tableTraceIDField.Append(t.TraceId)

		spans := t.GetSpans()
		if len(spans) < 1 {
			log.DefaultLogger.Warn("failed getting trace spans", "traceID", t.TraceId)
			continue
		}

		rootSpan := spans[0]
		tableTraceNameField.Append(cloudtrace.GetTraceName(rootSpan))
		tableStartTimeField.Append(rootSpan.GetStartTime().AsTime())
		latency := rootSpan.GetEndTime().AsTime().UnixMilli() - rootSpan.GetStartTime().AsTime().UnixMilli()
		tableLatencyField.Append(latency)
	}

	f.Fields = append(f.Fields,
		tableTraceIDField,
		tableTraceNameField,
		tableStartTimeField,
		tableLatencyField,
	)

	return f
}

// CheckHealth handles health checks sent from Grafana to the plugin.
// The main use case for these health checks is the test button on the
// datasource configuration page which allows users to verify that
// a datasource is working as expected.
func (d *CloudTraceDatasource) CheckHealth(ctx context.Context, req *backend.CheckHealthRequest) (*backend.CheckHealthResult, error) {
	// log.DefaultLogger.Info("CheckHealth called")

	client := d.client

	if d.oauthPassThrough {
		oauthClient, err := d.CreateOauthClient(ctx, req.Headers)
		if err != nil {
			return &backend.CheckHealthResult{
				Status:  backend.HealthStatusError,
				Message: sanitizeErrorMessage(err),
			}, nil
		}
		client = oauthClient
		defer client.Close()
	}

	var status = backend.HealthStatusOk
	settings := req.PluginContext.DataSourceInstanceSettings

	var conf config
	if err := json.Unmarshal(settings.JSONData, &conf); err != nil {
		return &backend.CheckHealthResult{
			Status:  backend.HealthStatusError,
			Message: fmt.Sprintf("failed to parse configuration: %s", sanitizeErrorMessage(err)),
		}, nil
	}
	if conf.DefaultProject == "" && conf.AuthType == gceAuthentication {
		proj, err := utils.GCEDefaultProject(ctx, "")
		if err != nil {
			return &backend.CheckHealthResult{
				Status:  backend.HealthStatusError,
				Message: fmt.Sprintf("failed to get GCE default project: %s", sanitizeErrorMessage(err)),
			}, nil
		}
		conf.DefaultProject = proj
	}
	if conf.DefaultProject == "" && conf.OAuthPassThru {
		return &backend.CheckHealthResult{
			Status:  backend.HealthStatusError,
			Message: "Please define a default project for OAuth authentication",
		}, nil
	}
	if err := client.TestConnection(ctx, conf.DefaultProject); err != nil {
		return &backend.CheckHealthResult{
			Status:  backend.HealthStatusError,
			Message: fmt.Sprintf("failed to run test query: %s", sanitizeErrorMessage(err)),
		}, nil
	}

	return &backend.CheckHealthResult{
		Status:  status,
		Message: fmt.Sprintf("Successfully queried traces from GCP project %s", conf.DefaultProject),
	}, nil
}

// htmlLikePattern matches error strings that contain HTML responses. It targets
// specific HTML signatures to avoid false positives from Go error messages that
// contain angle-bracket notation (e.g. <nil> from ASN.1/x509 parsing).
var htmlLikePattern = regexp.MustCompile(`(?i)<html[\s>]|<!doctype\s+html|text/html`)

// sanitizeErrorMessage cleans up raw error messages that may contain HTML
// (e.g. from an incorrect universe domain returning a 502 HTML page). It
// handles both the case where the full HTML page is embedded in the error and
// the case where only the content-type is mentioned (gRPC transport errors).
func sanitizeErrorMessage(err error) string {
	msg := err.Error()
	if !htmlLikePattern.MatchString(msg) {
		return msg
	}
	return "The server returned an HTML error page instead of a valid API response. " +
		"If you have configured a Universe Domain, please verify it is correct."
}

func (d *CloudTraceDatasource) CreateOauthClient(ctx context.Context, headers map[string]string) (*cloudtrace.Client, error) {
	client, err := cloudtrace.NewClientWithPassThrough(ctx, headers, d.universeDomain)
	if err != nil {
		return nil, fmt.Errorf("create oauth client: %s", sanitizeErrorMessage(err))
	}

	return client, nil
}
