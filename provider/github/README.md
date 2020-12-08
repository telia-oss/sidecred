# Github Provider

The Github provider supports the following credentials:
- `github:access-token`: An access token that can be used with the Github APIs, and for cloning over HTTPS.
- `github:deploy-key`: A deploy key that can be used to read/write from a specific repository.

See example configurations below.

### github:access-token

```yml
- type: github:access-token
  name: example
  config:
    owner: telia-oss
    permissions:
      contents: read
      statuses: write
      pull_requests: write
```

### github:deploy-key

```yml
- type: github:deploy-key
  name: example
  config:
    owner:
    repository: 
    title:
    read_only: true
```
