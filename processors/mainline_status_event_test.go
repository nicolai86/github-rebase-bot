package processors

import (
	"testing"

	"github.com/google/go-github/github"
)

func TestMainlineStatusEvent(t *testing.T) {
	ch := make(chan *github.StatusEvent, 1)

	t.Run("adds open PRs on mainline success", func(t *testing.T) {
		out := MainlineStatusEvent(Repository{
			Owner:    "test",
			Name:     "test",
			Mainline: "master",
		}, fakePullRequestResponse(2), ch)
		ch <- &github.StatusEvent{
			State: stringVal("success"),
			Branches: []*github.Branch{
				{
					Name: stringVal("master"),
				},
			},
			Repo: &github.Repository{
				Name: stringVal("test"),
				Owner: &github.User{
					Login: stringVal("test"),
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
