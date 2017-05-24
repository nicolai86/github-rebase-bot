package processors

import (
	"context"
	"fmt"

	"github.com/google/go-github/github"
)

// PushEvent emits every open PR once a change on mainline was received.
// This allows the bot to re-check all open PRs once master changed.
func PushEvent(repo Repository, client PullRequestLister, input <-chan *github.PushEvent) <-chan *github.PullRequest {
	ret := make(chan *github.PullRequest)
	go func() {
		for evt := range input {

			if evt.GetRef() != fmt.Sprintf("refs/heads/%s", repo.Mainline) {
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
