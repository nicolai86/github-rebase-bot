package main

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/go-github/github"
)

type fakePullRequestLister func() ([]*github.PullRequest, *github.Response, error)

func (f fakePullRequestLister) List(ctx context.Context, _ string, _ string, _ *github.PullRequestListOptions) ([]*github.PullRequest, *github.Response, error) {
	return f()
}

func TestProcessStatusEvent_Filters(t *testing.T) {
	for _, state := range []string{"pending", "failure", "error"} {
		t.Run(fmt.Sprintf("%s status", state), func(t *testing.T) {
			ch := make(chan *github.StatusEvent, 1)

			prs := processStatusEvent(nil, ch)
			ch <- &github.StatusEvent{
				State: stringVal(state),
			}
			close(ch)

			if v, ok := (<-prs); ok || v != nil {
				t.Errorf("Expected %s status to be filtered", state)
			}
		})
	}

	t.Run("closed pull-requests", func(t *testing.T) {
		ch := make(chan *github.StatusEvent, 1)

		prs := processStatusEvent(fakePullRequestLister(func() ([]*github.PullRequest, *github.Response, error) {
			return []*github.PullRequest{}, nil, nil
		}), ch)
		ch <- &github.StatusEvent{
			State: stringVal("success"),
			Branches: []*github.Branch{
				{Name: stringVal("test")},
			},
		}
		close(ch)

		if v, ok := (<-prs); ok || v != nil {
			t.Error("Expected closed pull requests to be filtered")
		}
	})
}

func TestProcessStatusEvent_PassThrough(t *testing.T) {
	ch := make(chan *github.StatusEvent, 1)

	prs := processStatusEvent(fakePullRequestLister(func() ([]*github.PullRequest, *github.Response, error) {
		return []*github.PullRequest{
			{
				Head: &github.PullRequestBranch{
					Ref: stringVal("test"),
				},
			},
		}, nil, nil
	}), ch)
	ch <- &github.StatusEvent{
		State: stringVal("success"),
		Branches: []*github.Branch{
			{Name: stringVal("test")},
		},
	}
	close(ch)

	if v, ok := (<-prs); !ok || v == nil {
		t.Error("Expected success status /w open PR to pass")
	}
}
