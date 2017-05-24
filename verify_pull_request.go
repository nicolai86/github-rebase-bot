package main

import (
	"context"
	"log"
	"strings"

	"github.com/google/go-github/github"
)

// PullRequestGetter queries github for a specific pull request
type PullRequestGetter interface {
	Get(context.Context, string, string, int) (*github.PullRequest, *github.Response, error)
}

// PullRequestLister queries github for all pull requests
type PullRequestLister interface {
	List(context.Context, string, string, *github.PullRequestListOptions) ([]*github.PullRequest, *github.Response, error)
}

// IssueGetter queries github for a specific issue
type IssueGetter interface {
	Get(context.Context, string, string, int) (*github.Issue, *github.Response, error)
}

type StatusGetter interface {
	GetCombinedStatus(context.Context, string, string, string, *github.ListOptions) (*github.CombinedStatus, *github.Response, error)
}

// verifyPullRequest filters out non-mergeable pull requests
func verifyPullRequest(issueClient IssueGetter, statusClient StatusGetter, mergeLabel string, input <-chan *github.PullRequest) <-chan *github.PullRequest {
	ret := make(chan *github.PullRequest)
	go func() {
		for pr := range input {
			if pr.GetState() != "open" {
				log.Printf("%s/%s: pr %d is %s.\n", pr.Base.Repo.Owner.GetLogin(), pr.Base.Repo.GetName(), pr.GetNumber(), pr.GetState())
				continue
			}

			issue, _, err := issueClient.Get(
				context.Background(),
				pr.Base.Repo.Owner.GetLogin(),
				pr.Base.Repo.GetName(),
				pr.GetNumber(),
			)
			if err != nil {
				log.Printf("%s/%s: pr %d failed to lookup issue %s.\n", pr.Base.Repo.Owner.GetLogin(), pr.Base.Repo.GetName(), pr.GetNumber(), err.Error())
				continue
			}

			mergeable := false
			for _, label := range issue.Labels {
				mergeable = mergeable || strings.EqualFold(*label.Name, mergeLabel)
			}

			if !mergeable || (pr.Mergeable != nil && !*pr.Mergeable) {
				log.Printf("%s/%s: pr %d is not mergeable.\n", pr.Base.Repo.Owner.GetLogin(), pr.Base.Repo.GetName(), pr.GetNumber())
				continue
			}

			status, _, err := statusClient.GetCombinedStatus(
				context.Background(),
				pr.Base.Repo.Owner.GetLogin(),
				pr.Base.Repo.GetName(),
				*pr.Head.SHA,
				&github.ListOptions{},
			)
			if err != nil {
				continue
			}

			if status.GetState() != "success" {
				log.Printf("%s/%s: pr %d status is %s.\n", pr.Base.Repo.Owner.GetLogin(), pr.Base.Repo.GetName(), pr.GetNumber(), status.GetState())
				continue
			}

			ret <- pr
		}
		close(ret)
	}()
	return ret
}
