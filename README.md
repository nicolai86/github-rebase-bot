# rebase-bot

[![wercker status](https://app.wercker.com/status/a36462ea15c0685dbb331bd23641faa7/s/master "wercker status")](https://app.wercker.com/project/byKey/a36462ea15c0685dbb331bd23641faa7)

the rebase-bot takes care of ensuring proper protocol is followed when working with
pull-requests.

specifically it…

… automatically rebases master once LGTM'd to ensure PR is up-to-date  
… waits for tests and merge automatically when green

## deployment

the rebase-bot is build with kubernetes (k8s) as a deployment target. Yet you could run it outside of a k8s cluster.
The following steps assume you have a running k8s cluster with RBAC enabled:

1. modify `k8s/deployment.yml` to pass along the correct list of `GITHUB_REPOS` 
   the syntax is `owner/repo:mainline`, e.g. `nicolai86/github-rebase-bot#master`.
   Multiple repositories can be separated by `,`.
2. modify `k8s/secrets.yml` and add base64 encoded github token.
3. apply k8s configuration: `kubectl apply -f k8s/`

this will create a `github` namespace with the bot running inside.

## development

The rebase-bot has lots of unit tests to ensure it's working as intended. Also
there are some integration tests which operate on a locally cloned repository with pre-defined scenarios.

To execute all tests run `go test ./...`.

For prototyping it's sometimes useful to run the bot locally. This is best done via [ngrok](https://ngrok.io):

1. start ngrok: `ngrok http 8080`
2. start the bot with the endpoint provided by ngrok: `go build . && ./github-rebase-bot -public-dns <ngrok-endpoint> -repos <repo> -merge-label LGTM -addr :8080`

To update the integration test scenarios unarchive `scenarios/rebase-conflict.zip`, change the repository and archive it again using zip: `zip -r ../rebase-conflict.zip .`

## installation

`go get -u github.com/nicolai86/github-rebase-bot`
