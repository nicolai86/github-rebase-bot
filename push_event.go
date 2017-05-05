package main

import (
	"context"
	"fmt"

	"github.com/google/go-github/github"
)

func processPushEvent(client PullRequestLister, input <-chan *github.PushEvent) <-chan *github.PullRequest {
	ret := make(chan *github.PullRequest)
	go func() {
		for evt := range input {

			if evt.GetRef() != fmt.Sprintf("refs/heads/%s", mainline) {
				continue
			}

			prs, _, err := client.List(
				context.Background(),
				owner,
				repository,
				&github.PullRequestListOptions{
					State: "open",
				})
			if err != nil {
				continue
			}
			for _, pr := range prs {
				ret <- pr
			}
		}

		close(ret)
	}()
	return ret
}
