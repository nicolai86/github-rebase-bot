package main

import (
	"context"
	"fmt"

	"github.com/google/go-github/github"
)

func processPushEvent(repo repository, client PullRequestLister, input <-chan *github.PushEvent) <-chan *github.PullRequest {
	ret := make(chan *github.PullRequest)
	go func() {
		for evt := range input {

			if evt.GetRef() != fmt.Sprintf("refs/heads/%s", repo.mainline) {
				continue
			}

			prs, _, err := client.List(
				context.Background(),
				repo.owner,
				repo.name,
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
