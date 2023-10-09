# Google Cloud Trace Data Source

## Overview
The Google Cloud Trace Data Source is a backend data source plugin for Grafana,
which allows users to query and visualize their Google Cloud traces and spans in Grafana.

![image info](https://github.com/GoogleCloudPlatform/cloud-trace-data-source-plugin/blob/main/src/img/cloud_trace_explore_view.png?raw=true)

## Supported Grafana Version
9.0.x+

## Setup

### Download
Download this plugin to the machine Grafana is running on, either using `git clone` or simply downloading it as a ZIP file. For the purpose of this guide, we'll assume the user "alice" has downloaded it into their local directory "/Users/alice/grafana/". If you are running the Grafana server using a user such as `grafana`, make sure the user has access to the directory.

### Enable Cloud Resource Manager API

You need to enable the resource manager API. Otherwise, your cloud projects will not be displayed in the dropdown menu.

You can follow the steps to enable it:

1. Navigate to the [cloud resource manager API page](https://console.cloud.google.com/apis/library/cloudresourcemanager.googleapis.com) in GCP and select your project
2. Press the `Enable` button

### Generate a JWT file & Assign IAM Permissions

1. If you don't have gcp project, add a new gcp project. [link](https://cloud.google.com/resource-manager/docs/creating-managing-projects#console)
2. Open the [Credentials](https://console.developers.google.com/apis/credentials) page in the Google API Console
3. Click **Create Credentials** then click **Service account**
4. On the Create service account page, enter the Service account details
5. On the `Create service account` page, fill in the `Service account details` and then click `Create and Continue`
6. On the `Grant this service account access to project` section, add the `Cloud Trace User` role under `Cloud Trace` to the service account. Click `Done`
7. In the next step, click the service account you just created. Under the `Keys` tab and select `Add key` and `Create new key`
8. Choose key type `JSON` and click `Create`. A JSON key file will be created and downloaded to your computer

If you want to access traces in multiple cloud projects, you need to ensure the service account has permission to read logs from all of them.

If you host Grafana on a GCE VM, you can also use the [Compute Engine service account](https://cloud.google.com/compute/docs/access/service-accounts#serviceaccount). You need to make sure the service account has sufficient permissions to access the traces in all projects.

### Service account impersonation
You can also configure the plugin to use [service account impersonation](https://cloud.google.com/iam/docs/service-account-impersonation).
You need to ensure the service account used by this plugin has the `iam.serviceAccounts.getAccessToken` permission. This permission is in roles like the [Service Account Token Creator role](https://cloud.google.com/iam/docs/understanding-roles#iam.serviceAccountTokenCreator) (roles/iam.serviceAccountTokenCreator). Also, the service account impersonated
by this plugin needs cloud trace user and project list permissions.
### Grafana Configuration
1. With Grafana restarted, navigate to `Configuration -> Data sources` (or the route `/datasources`)
2. Click "Add data source"
3. Select "Google Cloud Trace"
4. Provide credentials in a JWT file, either by using the file selector or pasting the contents of the file.
5. Click "Save & test" to test that traces can be queried from Cloud Trace.

![image info](https://github.com/GoogleCloudPlatform/cloud-trace-data-source-plugin/blob/main/src/img/cloud_trace_config.png?raw=true)


## Usage

### Grafana Explore
1. After configuration, navigate to `Explore` (or the route `/explore`).
2. Select "Google Cloud Trace" from the dropdown list of datasources.
3. Select either `Filter` or `Trace ID` for the query type.
4. For `Trace ID` queries, simply enter in a trace ID to view the trace and its associated spans.
5. For `Filter` queries, enter any number of filters in the form of `[key]:[value]`. 
   Typically these filters are are used to match labels on the traces. These filters are additive.
   There are also a number of special user friendly keys you can use:
    - `RootSpan` matches any trace which contains the given root span name
    - `SpanName` matches any trace which contains the given span name
    - `HasLabel` matches any trace which contains the given label key
	- `MinLatency` matches any trace which has a latency greater than the given latency
	- `Version` matches any trace which contains the label `g.co/gae/app/version` with the given service version
	- `Service` matches any trace which contains the label `g.co/gae/app/module` with the given service name
	- `Status` matches any trace which contains the label `/http/status_code` with the given status
	- `URL` matches any trace which contains the label `/http/url` with the given url
	- `Method` matches any trace which contains the label `/http/method` with the given HTTP method

    After making a `Filter` query, a table will be displayed with all of the matching traces
    (Example: `http.scheme:http http.server_name:testserver MinLatency:500ms`)

### Supported variables
The plugin currently supports variables for the GCP projects and a trace id. The project variable is a query one, and the trace id is a text or custom one.

## Licenses
Cloud Trace Logo (`src/img/logo.svg`) is from Google Cloud's [Official icons and sample diagrams](https://cloud.google.com/icons)

As commented, `JWTForm` and `JWTConfigEditor` are largely based on Apache-2.0 licensed [grafana-google-sdk-react](https://github.com/grafana/grafana-google-sdk-react/)
