package processors

import (
	"errors"
	"sync"

	"github.com/google/go-github/github"
	"github.com/nicolai86/github-rebase-bot/repo"
)

type RebaseResult struct {
	PR    *github.PullRequest
	Error error
}

var ErrMainlineChanged = errors.New("mainline changed during rebase")

// Rebase rebases a pull request with mainline.
// if the rebase is possible the changes are pushed to github.
// when no rebase was necessary the PR is emitted
func Rebase(r Repository, in <-chan *github.PullRequest) <-chan RebaseResult {
	ret := make(chan RebaseResult)

	go func() {
		wg := sync.WaitGroup{}

		for pr := range in {
			cache := r.Cache
			w, err := cache.Worker(pr.Head.GetRef())
			if err != nil {
				ret <- RebaseResult{pr, err}
				continue
			}

			rev, err := cache.Update()
			if err != nil {
				ret <- RebaseResult{pr, err}
				continue
			}

			c := make(chan repo.Signal, 1)
			wg.Add(1)
			w.Enqueue(c)
			go func(pr *github.PullRequest, rev string) {
				defer wg.Done()
				sig := <-c

				rev2, _ := cache.Update()
				if rev != rev2 {
					// mainline changed while we were processing this PR. re-process to handle cont. rebasing
					ret <- RebaseResult{pr, ErrMainlineChanged}
					return
				}

				if sig.Error != nil {
					ret <- RebaseResult{pr, sig.Error}
					return
				}
				if sig.UpToDate {
					ret <- RebaseResult{pr, nil}
				}
			}(pr, rev)
		}

		wg.Wait()
		close(ret)
	}()

	return ret
}
