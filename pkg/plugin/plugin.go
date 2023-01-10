package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/instancemgmt"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	cloudtrace "github.com/observiq/cloud-trace-grafana-ds/pkg/plugin/cloudtrace"
)

// Make sure CloudTraceDatasource implements required interfaces
var (
	_                     backend.QueryDataHandler      = (*CloudTraceDatasource)(nil)
	_                     backend.CheckHealthHandler    = (*CloudTraceDatasource)(nil)
	_                     instancemgmt.InstanceDisposer = (*CloudTraceDatasource)(nil)
	errMissingCredentials                               = errors.New("missing credentials")
)

const (
	privateKeyKey = "privateKey"
)

// config is the fields parsed from the front end
type config struct {
	AuthType       string `json:"authenticationType"`
	ClientEmail    string `json:"clientEmail"`
	DefaultProject string `json:"defaultProject"`
	TokenURI       string `json:"tokenUri"`
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
func NewCloudTraceDatasource(settings backend.DataSourceInstanceSettings) (instancemgmt.Instance, error) {
	var conf config
	if err := json.Unmarshal(settings.JSONData, &conf); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}

	privateKey, ok := settings.DecryptedSecureJSONData[privateKeyKey]
	if !ok || privateKey == "" {
		return nil, errMissingCredentials
	}

	serviceAccount, err := conf.toServiceAccountJSON(privateKey)
	if err != nil {
		return nil, fmt.Errorf("create credentials: %w", err)
	}

	client, err := cloudtrace.NewClient(context.TODO(), serviceAccount)
	if err != nil {
		return nil, err
	}

	return &CloudTraceDatasource{
		client: client,
	}, nil
}

// CloudTraceDatasource is an example datasource which can respond to data queries, reports
// its health and has streaming skills.
type CloudTraceDatasource struct {
	client cloudtrace.API
}

// Dispose here tells plugin SDK that plugin wants to clean up resources when a new instance
// created. As soon as datasource settings change detected by SDK old datasource instance will
// be disposed and a new one will be created using NewSampleDatasource factory function.
func (d *CloudTraceDatasource) Dispose() {
	if err := d.client.Close(); err != nil {
		log.DefaultLogger.Error("failed closing client", "error", err)
	}
}

// ListProjectsResponse is our response to a call to `/resources/projects`
type ListProjectsResponse struct {
	Projects []string `json:"projects"`
}

// CallResource fetches some resource from GCP using the data source's credentials
//
// Currently only projects are fetched, other requests receive a 404
func (d *CloudTraceDatasource) CallResource(ctx context.Context, req *backend.CallResourceRequest, sender backend.CallResourceResponseSender) error {
	log.DefaultLogger.Info("CallResource called")

	// Right now we only support calls to `/projects`
	resource := req.Path
	if strings.ToLower(resource) != "projects" {
		return sender.Send(&backend.CallResourceResponse{
			Status: http.StatusNotFound,
			Body:   []byte(`No such path`),
		})
	}

	projects, err := d.client.ListProjects(ctx)
	if err != nil {
		log.DefaultLogger.Error("error listing", "error", err)
		return sender.Send(&backend.CallResourceResponse{
			Status: http.StatusInternalServerError,
			Body:   []byte(`Unable to list projects`),
		})
	}

	body, err := json.Marshal(&ListProjectsResponse{Projects: projects})
	if err != nil {
		return sender.Send(&backend.CallResourceResponse{
			Status: http.StatusInternalServerError,
			Body:   []byte(`Unable to create response`),
		})
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
	log.DefaultLogger.Info("QueryData called")

	// create response struct
	response := backend.NewQueryDataResponse()

	// loop over queries and execute them individually.
	for _, q := range req.Queries {
		res := d.query(ctx, req.PluginContext, q)

		// save the response in a hashmap
		// based on with RefID as identifier
		response.Responses[q.RefID] = res
	}

	return response, nil
}

// queryModel is the fields needed to query from Grafana
type queryModel struct {
	QueryText     string `json:"queryText"`
	ProjectID     string `json:"projectId"`
	MaxDataPoints int    `json:"MaxDataPoints"`
}

func (d *CloudTraceDatasource) query(ctx context.Context, pCtx backend.PluginContext, query backend.DataQuery) backend.DataResponse {
	return backend.DataResponse{}
}

// CheckHealth handles health checks sent from Grafana to the plugin.
// The main use case for these health checks is the test button on the
// datasource configuration page which allows users to verify that
// a datasource is working as expected.
func (d *CloudTraceDatasource) CheckHealth(ctx context.Context, req *backend.CheckHealthRequest) (*backend.CheckHealthResult, error) {
	log.DefaultLogger.Info("CheckHealth called")

	var status = backend.HealthStatusOk
	settings := req.PluginContext.DataSourceInstanceSettings

	var conf config
	if err := json.Unmarshal(settings.JSONData, &conf); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}

	privateKey, ok := settings.DecryptedSecureJSONData[privateKeyKey]
	if !ok || privateKey == "" {
		return nil, errMissingCredentials
	}

	serviceAccount, err := conf.toServiceAccountJSON(privateKey)
	if err != nil {
		return nil, fmt.Errorf("create credentials: %w", err)
	}

	client, err := cloudtrace.NewClient(ctx, serviceAccount)
	if err != nil {
		return nil, err
	}

	defer func() {
		if err := client.Close(); err != nil {
			log.DefaultLogger.Warn("failed closing client", "error", err)
		}
	}()

	if err := client.TestConnection(ctx, conf.DefaultProject); err != nil {
		return &backend.CheckHealthResult{
			Status:  backend.HealthStatusError,
			Message: fmt.Sprintf("failed to run test query: %s", err),
		}, nil
	}

	return &backend.CheckHealthResult{
		Status:  status,
		Message: fmt.Sprintf("Successfully queried traces from GCP project %s", conf.DefaultProject),
	}, nil
}
