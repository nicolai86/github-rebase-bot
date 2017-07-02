package processors

import (
	"errors"
	"sync"
	"testing"

	"github.com/google/go-github/github"
	"github.com/nicolai86/github-rebase-bot/repo"
)

type fakeWorkerCache func(string) (repo.Enqueuer, error)

func (f fakeWorkerCache) Worker(branch string) (repo.Enqueuer, error) {
	return f(branch)
}

func (f fakeWorkerCache) Update() (string, error) {
	return "", nil
}
func (f fakeWorkerCache) Cleanup(v repo.GitWorktree) error {
	return nil
}

type fakeEnqueuer func() repo.Signal

func (f fakeEnqueuer) Enqueue(c chan repo.Signal) {
	c <- f()
	close(c)
}

func TestRebase(t *testing.T) {
	t.Run("requests a branch specific worker", func(t *testing.T) {
		ch := make(chan *github.PullRequest)
		prBranch := "super-feature"
		prNumber := 2202
		var wg sync.WaitGroup
		wg.Add(1)
		r := Repository{
			Owner:    "test",
			Name:     "test",
			Mainline: "master",
			Cache: fakeWorkerCache(func(branch string) (repo.Enqueuer, error) {
				wg.Done()
				if prBranch != branch {
					t.Fatalf("Expected branch %q but got %q ", prBranch, branch)
				}
				return nil, errors.New("failed to checkout repo")
			}),
		}
		Rebase(r, ch)
		ch <- &github.PullRequest{
			Number: intVal(prNumber),
			Head: &github.PullRequestBranch{
				Ref: stringVal(prBranch),
			},
			Base: &github.PullRequestBranch{
				Repo: &github.Repository{
					Name: stringVal("test"),
					Owner: &github.User{
						Login: stringVal("test"),
					},
				},
			},
		}
		close(ch)
		wg.Wait()
	})

	t.Run("filters when worker fetching errors", func(t *testing.T) {
		ch := make(chan *github.PullRequest)
		r := Repository{
			Owner:    "test",
			Name:     "test",
			Mainline: "master",
			Cache: fakeWorkerCache(func(_ string) (repo.Enqueuer, error) {
				return nil, errors.New("failed to checkout repo")
			}),
		}
		ret := Rebase(r, ch)
		ch <- &github.PullRequest{
			Base: &github.PullRequestBranch{
				Repo: &github.Repository{
					Name: stringVal("test"),
					Owner: &github.User{
						Login: stringVal("test"),
					},
				},
			},
		}
		close(ch)
		if v, ok := (<-ret); v.Error == nil || !ok {
			t.Fatal("Expected pull request to be skipped")
		}
	})

	t.Run("filters rebased branches", func(t *testing.T) {
		ch := make(chan *github.PullRequest)
		ret := Rebase(Repository{
			Owner:    "test",
			Name:     "test",
			Mainline: "master",
			Cache: fakeWorkerCache(func(branch string) (repo.Enqueuer, error) {
				return fakeEnqueuer(func() repo.Signal { return repo.Signal{UpToDate: false} }), nil
			}),
		}, ch)
		ch <- &github.PullRequest{
			Base: &github.PullRequestBranch{
				Repo: &github.Repository{
					Name: stringVal("test"),
					Owner: &github.User{
						Login: stringVal("test"),
					},
				},
			},
		}
		close(ch)
		if v, ok := (<-ret); v.Error != nil || ok {
			t.Fatal("Expected pull request to be skipped")
		}
	})

	t.Run("filters error'd branches", func(t *testing.T) {
		ch := make(chan *github.PullRequest)
		ret := Rebase(Repository{
			Owner:    "test",
			Name:     "test",
			Mainline: "master",
			Cache: fakeWorkerCache(func(branch string) (repo.Enqueuer, error) {
				return fakeEnqueuer(func() repo.Signal { return repo.Signal{Error: errors.New("git: unknown binary")} }), nil
			}),
		}, ch)
		ch <- &github.PullRequest{
			Base: &github.PullRequestBranch{
				Repo: &github.Repository{
					Name: stringVal("test"),
					Owner: &github.User{
						Login: stringVal("test"),
					},
				},
			},
		}
		close(ch)
		if v, ok := (<-ret); v.Error == nil || !ok {
			t.Fatal("Expected pull request to be skipped")
		}
	})

	t.Run("passes through up2date branches", func(t *testing.T) {
		ch := make(chan *github.PullRequest)
		ret := Rebase(Repository{
			Owner:    "test",
			Name:     "test",
			Mainline: "master",
			Cache: fakeWorkerCache(func(branch string) (repo.Enqueuer, error) {
				return fakeEnqueuer(func() repo.Signal { return repo.Signal{UpToDate: true} }), nil
			}),
		}, ch)
		ch <- &github.PullRequest{
			Base: &github.PullRequestBranch{
				Repo: &github.Repository{
					Name: stringVal("test"),
					Owner: &github.User{
						Login: stringVal("test"),
					},
				},
			},
		}
		close(ch)
		if v, ok := (<-ret); v.Error != nil || !ok {
			t.Fatal("Expected pull request to pass through")
		}
	})
}
