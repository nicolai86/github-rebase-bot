package processors

import (
	"context"

	"github.com/google/go-github/github"
)

// MainlineStatusEvent takes mainline status events and emits open PRs
func MainlineStatusEvent(repo Repository, client PullRequestLister, input <-chan *github.StatusEvent) <-chan *github.PullRequest {
	ret := make(chan *github.PullRequest)
	go func() {
		for evt := range input {
			if evt.GetState() != "success" {
				continue
			}

			isMainline := false
			for _, branch := range evt.Branches {
				isMainline = isMainline || *branch.Name == repo.Mainline
			}

			if !isMainline {
				continue
			}

			prs, _, err := client.List(
				context.Background(),
				repo.Owner,
				repo.Name,
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
