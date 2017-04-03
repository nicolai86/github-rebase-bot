package main

import (
	"context"
	"testing"

	"github.com/google/go-github/github"
)

type fakeIssueGetter func() (*github.Issue, *github.Response, error)

func (f fakeIssueGetter) Get(ctx context.Context, _ string, _ string, _ int) (*github.Issue, *github.Response, error) {
	return f()
}

type fakeStatusGetter func() (*github.CombinedStatus, *github.Response, error)

func (f fakeStatusGetter) GetCombinedStatus(ctx context.Context, _ string, _ string, _ string, _ *github.ListOptions) (*github.CombinedStatus, *github.Response, error) {
	return f()
}

func TestVerifyPullRequest_Filters(t *testing.T) {
	mergeLabel := "Ready to Merge"
	t.Run("closed pull-requests", func(t *testing.T) {
		ch := make(chan *github.PullRequest, 1)

		prs := verifyPullRequest(nil, nil, mergeLabel, ch)
		ch <- &github.PullRequest{
			State:  stringVal("closed"),
			Number: intVal(1),
		}
		close(ch)

		if v, ok := (<-prs); ok || v != nil {
			t.Error("Expected closed pull-requests to be filtered")
		}
	})

	t.Run("open pull-requests w/o merge label", func(t *testing.T) {
		ch := make(chan *github.PullRequest, 1)

		prs := verifyPullRequest(fakeIssueGetter(func() (*github.Issue, *github.Response, error) {
			return &github.Issue{
				Labels: []github.Label{
					{Name: stringVal("LGTM")},
				},
			}, nil, nil
		}), nil, mergeLabel, ch)
		ch <- &github.PullRequest{
			State:  stringVal("open"),
			Number: intVal(1),
		}
		close(ch)

		if v, ok := (<-prs); ok || v != nil {
			t.Error("Expected open pull-requests w/o matching label to be filtered")
		}
	})

	t.Run("open pull-requests /w conflict", func(t *testing.T) {
		ch := make(chan *github.PullRequest, 1)

		issueClient := fakeIssueGetter(func() (*github.Issue, *github.Response, error) {
			return &github.Issue{
				Labels: []github.Label{
					{Name: stringVal(mergeLabel)},
				},
			}, nil, nil
		})
		statusClient := fakeStatusGetter(func() (*github.CombinedStatus, *github.Response, error) {
			return &github.CombinedStatus{
				State: stringVal("success"),
			}, nil, nil
		})
		prs := verifyPullRequest(issueClient, statusClient, mergeLabel, ch)
		ch <- &github.PullRequest{
			State:  stringVal("open"),
			Number: intVal(1),
			Head: &github.PullRequestBranch{
				Ref: stringVal("test"),
				SHA: stringVal("098f6bcd4621d373cade4e832627b4f6"),
			},
			Mergeable: boolVal(false),
		}
		close(ch)

		if v, ok := (<-prs); ok || v != nil {
			t.Error("Expected open pull-requests w/o matching label to be filtered")
		}
	})

	t.Run("open pull-requests /w non-success status", func(t *testing.T) {
		ch := make(chan *github.PullRequest, 1)

		issueClient := fakeIssueGetter(func() (*github.Issue, *github.Response, error) {
			return &github.Issue{
				Labels: []github.Label{
					{Name: stringVal(mergeLabel)},
				},
			}, nil, nil
		})
		statusClient := fakeStatusGetter(func() (*github.CombinedStatus, *github.Response, error) {
			return &github.CombinedStatus{
				State: stringVal("failure"),
			}, nil, nil
		})
		prs := verifyPullRequest(issueClient, statusClient, mergeLabel, ch)
		ch <- &github.PullRequest{
			State:  stringVal("open"),
			Number: intVal(1),
			Head: &github.PullRequestBranch{
				Ref: stringVal("test"),
				SHA: stringVal("098f6bcd4621d373cade4e832627b4f6"),
			},
			Mergeable: boolVal(true),
		}
		close(ch)

		if v, ok := (<-prs); ok || v != nil {
			t.Error("Expected open pull-requests w/o matching label to be filtered")
		}
	})
}

func TestVerifyPullRequest_PassThrough(t *testing.T) {
	mergeLabel := "Ready to Merge"
	ch := make(chan *github.PullRequest, 1)

	issueClient := fakeIssueGetter(func() (*github.Issue, *github.Response, error) {
		return &github.Issue{
			Labels: []github.Label{
				{Name: stringVal(mergeLabel)},
			},
		}, nil, nil
	})
	statusClient := fakeStatusGetter(func() (*github.CombinedStatus, *github.Response, error) {
		return &github.CombinedStatus{
			State: stringVal("success"),
		}, nil, nil
	})

	prs := verifyPullRequest(issueClient, statusClient, mergeLabel, ch)
	ch <- &github.PullRequest{
		State:  stringVal("open"),
		Number: intVal(1),
		Head: &github.PullRequestBranch{
			Ref: stringVal("test"),
			SHA: stringVal("098f6bcd4621d373cade4e832627b4f6"),
		},
		Mergeable: boolVal(true),
	}
	close(ch)

	if v, ok := (<-prs); !ok || v == nil {
		t.Error("Expected open pull-requests w/ matching label to pass")
	}
}
