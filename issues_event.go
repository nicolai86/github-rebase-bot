package main

import (
	"context"

	"github.com/google/go-github/github"
)

// processIssuesEvent filters out issues and allows pull requests
func processIssuesEvent(client PullRequestGetter, input <-chan *github.IssuesEvent) <-chan *github.PullRequest {
	ret := make(chan *github.PullRequest)
	go func() {
		for evt := range input {
			pr, _, err := client.Get(
				context.Background(),
				owner,
				repository,
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
