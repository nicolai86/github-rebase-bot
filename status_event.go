package main

import (
	"context"

	"github.com/google/go-github/github"
)

type statusEventBroadcaster struct {
	listeners []chan<- *github.StatusEvent
}

func (b *statusEventBroadcaster) Listen(in <-chan *github.StatusEvent) {
	for evt := range in {
		for _, l := range b.listeners {
			l <- evt
		}
	}

	for _, l := range b.listeners {
		close(l)
	}
}

// processMainlineStatusEvent takes mainline status events and emits open PRs
func processMainlineStatusEvent(client PullRequestLister, input <-chan *github.StatusEvent) <-chan *github.PullRequest {
	ret := make(chan *github.PullRequest)
	go func() {
		for evt := range input {
			if evt.GetState() != "success" {
				continue
			}

			isMainline := false
			for _, branch := range evt.Branches {
				isMainline = isMainline || *branch.Name == mainline
			}

			if !isMainline {
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

// processStatusEvent filters out non-successful commits to PRs
func processStatusEvent(client PullRequestLister, input <-chan *github.StatusEvent) <-chan *github.PullRequest {
	ret := make(chan *github.PullRequest)

	go func() {
		for evt := range input {
			if evt.GetState() != "success" {
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

			var pr *github.PullRequest
			for _, branch := range evt.Branches {
				for _, p := range prs {
					if *p.Head.Ref == *branch.Name {
						pr = p
						break
					}
				}

				if pr != nil {
					break
				}
			}

			if pr == nil {
				continue
			}
			ret <- pr
		}
		close(ret)
	}()

	return ret
}
