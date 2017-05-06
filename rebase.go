package main

import (
	"log"
	"sync"

	"github.com/google/go-github/github"
	"github.com/nicolai86/github-rebase-bot/repo"
)

type WorkerCache interface {
	Worker(string) (repo.Enqueuer, error)
	Update() (string, error)
}

func processRebase(cache WorkerCache, in <-chan *github.PullRequest) <-chan *github.PullRequest {
	ret := make(chan *github.PullRequest)

	input := make(chan *github.PullRequest)
	go func() {
		for pr := range in {
			input <- pr
		}
		close(input)
	}()

	go func() {
		wg := sync.WaitGroup{}
		for pr := range input {
			w, err := cache.Worker(pr.Head.GetRef())
			if err != nil {
				continue
			}

			c := make(chan repo.Signal, 1)

			rev, err := cache.Update()
			if err != nil {
				log.Printf("failed to update %q: %v", mainline, err)
				continue
			}

			w.Enqueue(c)
			wg.Add(1)
			go func(pr *github.PullRequest, rev string) {
				defer wg.Done()
				sig := <-c

				rev2, _ := cache.Update()
				if rev != rev2 {
					// mainline changed while we were processing this PR. re-process to handle cont. rebasing
					input <- pr
					return
				}

				if sig.UpToDate && sig.Error == nil {
					ret <- pr
				}
			}(pr, rev)
		}

		wg.Wait()
		close(ret)
	}()

	return ret
}
