package main

import (
	"testing"

	"github.com/google/go-github/github"
)

func TestProcessPushEvent(t *testing.T) {
	ch := make(chan *github.PushEvent, 1)

	t.Run("adds open PRs on mainline push", func(t *testing.T) {
		mainline = "master"
		out := processPushEvent(fakePullRequestResponse(2), ch)
		ch <- &github.PushEvent{
			Ref: stringVal("refs/heads/master"),
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
