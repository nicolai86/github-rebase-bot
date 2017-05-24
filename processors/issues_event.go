package processors

import (
	"context"

	"github.com/google/go-github/github"
)

// IssuesEvent filters out events on issues which are not pull requests
func IssuesEvent(client PullRequestGetter, input <-chan *github.IssuesEvent) <-chan *github.PullRequest {
	ret := make(chan *github.PullRequest)
	go func() {
		for evt := range input {
			pr, _, err := client.Get(
				context.Background(),
				evt.Repo.Owner.GetLogin(),
				evt.Repo.GetName(),
				evt.Issue.GetNumber())
			if pr == nil || err != nil {
				continue
			}
			ret <- pr
		}
		close(ret)
	}()
	return ret
}
