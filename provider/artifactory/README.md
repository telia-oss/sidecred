# Artifactory Provider

This provider is used to generate time limited access tokens for access to [Artifactory](https://jfrog.com/artifactory/).

See the [package documentation](https://godoc.org/github.com/telia-oss/sidecred/provider/artifactory) for more information.

### Environment / Options

The following table shows the environment variables available to this provider.

| Variable | Type | Optional | Default | Description |
| -------- | ---- | -------- | ------- | ----------- |
| SIDECRED_ARTIFACTORY_PROVIDER_ENABLED | Bool | Yes | False | Flag to enable this provider |
| SIDECRED_ARTIFACTORY_PROVIDER_HOSTNAME | String | No | N/A | Artifactory endpoint (e.g., `https://my-org.jfrog.io/my-org/`) |
| SIDECRED_ARTIFACTORY_PROVIDER_USERNAME | String | No | N/A | REST API authentication username |
| SIDECRED_ARTIFACTORY_PROVIDER_PASSWORD | String | Yes | N/A | REST API authentication password |
| SIDECRED_ARTIFACTORY_PROVIDER_ACCESS_TOKEN | String | Yes | N/A | REST API access token |
| SIDECRED_ARTIFACTORY_PROVIDER_API_KEY | String | Yes | N/A | REST API key |
| SIDECRED_ARTIFACTORY_PROVIDER_SESSION_DURATION | Duration | Yes | `1h` | Default duration for generated tokens (`<= `1h`) |

The fields marked as not optional assume that the provider is enabled.

### Request

See the [official documentation](https://godoc.org/github.com/telia-oss/sidecred/provider/artifactory/#RequestConfig) for request configuration.
