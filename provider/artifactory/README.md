# Artifactory Provider

This provider is used to generate time limited access tokens for access to [Artifactory](https://jfrog.com/artifactory/).

## Overview

The provider excercises the REST API to generate time limited [access tokens](https://www.jfrog.com/confluence/display/JFROG/Access+Tokens). To access the API, the provider iteslef must be authenticated.

Artifactory's REST API generally supports [these authentication models](https://www.jfrog.com/confluence/display/JFROG/Artifactory+REST+API). Generally, this means we can authenticate with a dedicated username and password, where the password is one of the following:

* API Key
* Password
* Access token

The third is most desirable, as it means that we can allocate a revocable token under a specific username. Furthermore, that username can be a user allocated in Artifactory itself, as part of the call to issue a token. This avoids having to put an admin user's personal credentials into sidecred, or the API key, which have a higher blast radius if leaked.

## Configuration

### Environment / Options

The following table shows the environment variables available to this provider. These map to options in the [kingpin](`https://github.com/alecthomas/kingpin`) application, and can alternatively be specified on the command line for the application, if being run locally.

| Variable | Type | Optional | Default | Description |
| -------- | ---- | -------- | ------- | ----------- |
| SIDECRED_ARTIFACTORY_PROVIDER_ENABLED | Bool | Yes | False | Flag to enable this provider |
| SIDECRED_ARTIFACTORY_PROVIDER_HOSTNAME | String | No | N/A | Artifactory endpoint (e.g., `https://my-org.jfrog.io/my-org/`) |
| SIDECRED_ARTIFACTORY_PROVIDER_USERNAME | String | No | N/A | REST API authentication username |
| SIDECRED_ARTIFACTORY_PROVIDER_PASSWORD | String | Yes | N/A | REST API authentication password |
| SIDECRED_ARTIFACTORY_PROVIDER_ACCESS_TOKEN | String | Yes | N/A | REST API access token |
| SIDECRED_ARTIFACTORY_PROVIDER_API_KEY | String | Yes | N/A | REST API key |
| SIDECRED_ARTIFACTORY_PROVIDER_SESSION_DURATION | Duration | Yes | `1h` | Default duration for generate tokens (`<= `h`) |

The fields marked as not optional assume that the provider is enabled.

### Resource

| Field | Optional | Default | Description |
| ----- | -------- | ------- | ----------- |
| type | No | N/A | Must be set to `artifactory:access-token` |
| name | No | N/A | Resource name; used as prefix for secret name |
| config.user | No | N/A | User to create token for |
| config.group | No | N/A | Group to assign user to |
| config.duration | Yes | `1h` | Duration for token (`<= 1h>`) |


The generated secrets will be `<name>-artifactory-user` and `<name>-artifactory-token`.

The following shows an example resource configuration as YAML (note that the lambda version expects JSON):

```yaml
- type: artifactory:access-token
  name: my-writer
  config:
    user: concourse-artifactory-user
    group: artifactory-writers-group
    duration: 30m
```

For this specific example, the provider will create the secrets `my-writer-artifactory-user` and `my-writer-artifactory-token`. The value within the `my-writer-artifactory-user` secret will be `concourse-artifactory-user`. The secret `my-writer-artifactory-token` will contain the raw token.
