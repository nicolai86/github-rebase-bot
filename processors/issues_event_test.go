package processors

import (
	"context"
	"testing"

	"github.com/google/go-github/github"
)

type fakePullRequestGetter func() (*github.PullRequest, *github.Response, error)

func (f fakePullRequestGetter) Get(ctx context.Context, _ string, _ string, _ int) (*github.PullRequest, *github.Response, error) {
	return f()
}

func TestIssuesEvent_Filter(t *testing.T) {
	notAPullRequest := fakePullRequestGetter(func() (*github.PullRequest, *github.Response, error) {
		return nil, nil, nil
	})
	evt := github.IssuesEvent{
		Issue: &github.Issue{
			Number: intVal(1),
		},
		Repo: &github.Repository{
			Name: stringVal("test"),
			Owner: &github.User{
				Login: stringVal("test"),
			},
		},
	}

	ch := make(chan *github.IssuesEvent, 1)

	prs := IssuesEvent(notAPullRequest, ch)
	ch <- &evt
	close(ch)

	if v, ok := (<-prs); ok || v != nil {
		t.Error("Expected issue to be filtered")
	}
}

func TestIssuesEvent_PassThrough(t *testing.T) {
	aPullRequest := fakePullRequestGetter(func() (*github.PullRequest, *github.Response, error) {
		return &github.PullRequest{
			Number: intVal(1),
		}, nil, nil
	})
	evt := github.IssuesEvent{
		Issue: &github.Issue{
			Number: intVal(1),
		},
		Repo: &github.Repository{
			Name: stringVal("test"),
			Owner: &github.User{
				Login: stringVal("test"),
			},
		},
	}

	ch := make(chan *github.IssuesEvent, 1)

	prs := IssuesEvent(aPullRequest, ch)
	ch <- &evt
	close(ch)

	if v, ok := (<-prs); !ok || v == nil {
		t.Error("Expected pull-requests to pass through")
	}
}
