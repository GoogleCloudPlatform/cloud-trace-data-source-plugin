# How to Contribute

We'd love to accept your patches and contributions to this project.

## Before you begin

### Sign our Contributor License Agreement

Contributions to this project must be accompanied by a
[Contributor License Agreement](https://cla.developers.google.com/about) (CLA).
You (or your employer) retain the copyright to your contribution; this simply
gives us permission to use and redistribute your contributions as part of the
project.

If you or your current employer have already signed the Google CLA (even if it
was for a different project), you probably don't need to do it again.

Visit <https://cla.developers.google.com/> to see your current agreements or to
sign a new one.

### Review our Community Guidelines

This project follows
[Google's Open Source Community Guidelines](https://opensource.google/conduct/).

## Contribution process

### Build the plugin

If you download the source, you need to build the plugin. Make sure you have all the prerequisites installed and configured:

- Grafana 11.2 or later
- Go 1.27.3 or later
- [Mage](https://magefile.org/)
- Node.js 20 or later
- Yarn 1 (classic)

Under the source directory, run the following commands:

```bash
# Frontend
yarn install --frozen-lockfile
yarn build

# Backend: build the plugin binaries for Linux, Windows and Darwin into the dist directory
mage -v
```

To update the Grafana plugin SDK for Go to the latest minor version:

```bash
go get -u github.com/grafana/grafana-plugin-sdk-go
go mod tidy
```

More details, please read [the doc](https://grafana.com/tutorials/build-a-data-source-backend-plugin/).

### Test the plugin

```bash
# Frontend type checks and tests
yarn typecheck
yarn test:ci

# Backend tests
go test ./pkg/...
```

### Grafana Configuration

To have Grafana detect this plugin and make it available for use, two entries must be modified in the Grafana config file. See [Grafana's Documentation](https://grafana.com/docs/grafana/latest/setup-grafana/configure-grafana/) for more details, including the default location of the config file depending on platform.

First, set `paths.plugins` to point to where this repo has been downloaded locally. The final build artifacts will be under the `dist` directory:

```ini
[paths]
plugins = /Users/alice/grafana/googlecloud-trace-datasource/dist
```

Next, update `plugins.allow_loading_unsigned_plugins` so that this plugin's ID is in the list:

```ini
[plugins]
allow_loading_unsigned_plugins = googlecloud-trace-datasource
```

With these settings updated, we can now restart Grafana and expect the plugin to be available. The specific command to restart Grafana will depend on what platform it's running on, with the various options documented by [Grafana](https://grafana.com/docs/grafana/latest/setup-grafana/start-restart-grafana/).

### Code Reviews

All submissions, including submissions by project members, require review. We
use GitHub pull requests for this purpose. Consult
[GitHub Help](https://help.github.com/articles/about-pull-requests/) for more
information on using pull requests.
