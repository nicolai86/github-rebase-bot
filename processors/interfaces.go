package processors

import (
	"context"

	"github.com/google/go-github/github"
	"github.com/nicolai86/github-rebase-bot/repo"
)

// PullRequestGetter queries github for a specific pull request
type PullRequestGetter interface {
	Get(context.Context, string, string, int) (*github.PullRequest, *github.Response, error)
}

// PullRequestLister queries github for all pull requests
type PullRequestLister interface {
	List(context.Context, string, string, *github.PullRequestListOptions) ([]*github.PullRequest, *github.Response, error)
}

type WorkerCache interface {
	Worker(string) (repo.Enqueuer, error)
	Update() (string, error)
	Cleanup(repo.GitWorktree) error
}
