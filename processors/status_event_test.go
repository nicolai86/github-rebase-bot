package processors

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

func fakePullRequestResponse(n int) fakePullRequestLister {
	return fakePullRequestLister(func() ([]*github.PullRequest, *github.Response, error) {
		reqs := []*github.PullRequest{}
		for i := 0; i < n; i++ {
			reqs = append(reqs, &github.PullRequest{
				Head: &github.PullRequestBranch{
					Ref: stringVal("test"),
				},
				Base: &github.PullRequestBranch{
					User: &github.User{
						Login: stringVal("test"),
					},
					Repo: &github.Repository{
						Name: stringVal("test"),
					},
				},
			})
		}
		return reqs, nil, nil
	})
}

func TestStatusEvent_Filters(t *testing.T) {
	for _, state := range []string{"pending", "failure", "error"} {
		t.Run(fmt.Sprintf("%s status", state), func(t *testing.T) {
			ch := make(chan *github.StatusEvent, 1)

			prs := StatusEvent(nil, ch)
			ch <- &github.StatusEvent{
				State: stringVal(state),
				Repo: &github.Repository{
					Name: stringVal("test"),
					Owner: &github.User{
						Login: stringVal("test"),
					},
				},
			}
			close(ch)

			if v, ok := (<-prs); ok || v != nil {
				t.Errorf("Expected %s status to be filtered", state)
			}
		})
	}

	t.Run("closed pull-requests", func(t *testing.T) {
		ch := make(chan *github.StatusEvent, 1)

		prs := StatusEvent(fakePullRequestLister(func() ([]*github.PullRequest, *github.Response, error) {
			return []*github.PullRequest{}, nil, nil
		}), ch)
		ch <- &github.StatusEvent{
			State: stringVal("success"),
			Branches: []*github.Branch{
				{Name: stringVal("test")},
			},
			Repo: &github.Repository{
				Name: stringVal("test"),
				Owner: &github.User{
					Login: stringVal("test"),
				},
			},
		}
		close(ch)

		if v, ok := (<-prs); ok || v != nil {
			t.Error("Expected closed pull requests to be filtered")
		}
	})
}

func TestStatusEvent_PassThrough(t *testing.T) {
	ch := make(chan *github.StatusEvent, 1)

	prs := StatusEvent(fakePullRequestResponse(1), ch)
	ch <- &github.StatusEvent{
		State: stringVal("success"),
		Branches: []*github.Branch{
			{Name: stringVal("test")},
		},
		Repo: &github.Repository{
			Name: stringVal("test"),
			Owner: &github.User{
				Login: stringVal("test"),
			},
		},
	}
	close(ch)

	if v, ok := (<-prs); !ok || v == nil {
		t.Error("Expected success status /w open PR to pass")
	}
}
