package main

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/google/go-github/github"
)

func TestStatusEventBroadcaster(t *testing.T) {
	inp := make(chan *github.StatusEvent)
	q1 := make(chan *github.StatusEvent)
	q2 := make(chan *github.StatusEvent)
	b := statusEventBroadcaster{listeners: []chan<- *github.StatusEvent{q1, q2}}
	go b.Listen(inp)
	defer close(inp)

	w := sync.WaitGroup{}
	w.Add(2)
	go func() {
		<-q1
		w.Done()
	}()
	go func() {
		<-q2
		w.Done()
	}()
	inp <- &github.StatusEvent{}
	w.Wait()
}

func TestProcessMainlineStatusEvent(t *testing.T) {
	ch := make(chan *github.StatusEvent, 1)

	t.Run("adds open PRs on mainline success", func(t *testing.T) {
		mainline = "master"
		out := processMainlineStatusEvent(fakePullRequestResponse(2), ch)
		ch <- &github.StatusEvent{
			State: stringVal("success"),
			Branches: []*github.Branch{
				{
					Name: stringVal("master"),
				},
			},
		}
		close(ch)

		if _, ok := <-out; !ok {
			t.Fatal("Expected output on PR, but didn't receive")
		}
		if _, ok := <-out; !ok {
			t.Fatal("Expected output on PR, but didn't receive")
		}
	})
}

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
			})
		}
		return reqs, nil, nil
	})
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

	prs := processStatusEvent(fakePullRequestResponse(1), ch)
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
