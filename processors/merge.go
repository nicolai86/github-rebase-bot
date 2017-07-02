package processors

import (
	"context"
	"fmt"

	"github.com/google/go-github/github"
)

// Merge executes a merge to mainline via the github api.
func Merge(client *github.Client, input <-chan *github.PullRequest) <-chan *github.PullRequest {
	ret := make(chan *github.PullRequest)
	go func() {
		for pr := range input {
			if _, _, err := client.PullRequests.Merge(
				context.Background(),
				pr.Base.User.GetLogin(),
				pr.Base.Repo.GetName(),
				pr.GetNumber(),
				"merge-bot merged",
				&github.PullRequestOptions{
					MergeMethod: "merge",
				}); err != nil {
				continue
			}

			if _, err := client.Git.DeleteRef(
				context.Background(),
				pr.Base.User.GetLogin(),
				pr.Base.Repo.GetName(),
				fmt.Sprintf("heads/%s", *pr.Head.Ref),
			); err != nil {
				fmt.Printf("Failed deleting branch: %q\n", err)
			}

			ret <- pr
		}
		close(ret)
	}()
	return ret
}
