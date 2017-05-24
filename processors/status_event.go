package processors

import (
	"context"

	"github.com/google/go-github/github"
)

// StatusEvent emits pull requests when activity occurs on the specific branch
func StatusEvent(client PullRequestLister, input <-chan *github.StatusEvent) <-chan *github.PullRequest {
	ret := make(chan *github.PullRequest)

	go func() {
		for evt := range input {
			if evt.GetState() != "success" {
				continue
			}

			prs, _, err := client.List(
				context.Background(),
				evt.Repo.Owner.GetLogin(),
				evt.Repo.GetName(),
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
