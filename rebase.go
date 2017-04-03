package main

import (
	"github.com/google/go-github/github"
	"github.com/nicolai86/github-rebase-bot/repo"
)

type WorkerCache interface {
	Worker(string, int) (repo.Enqueuer, error)
}

func processRebase(cache WorkerCache, input <-chan *github.PullRequest) <-chan *github.PullRequest {
	ret := make(chan *github.PullRequest)

	go func() {
		for pr := range input {
			w, err := cache.Worker(
				pr.Head.GetRef(),
				pr.GetNumber(),
			)
			if err != nil {
				continue
			}

			c := make(chan repo.Signal, 1)
			w.Enqueue(c)
			go func(pr *github.PullRequest) {
				sig := <-c
				if sig.UpToDate && sig.Error == nil {
					ret <- pr
				}
				close(c)
			}(pr)
		}

		close(ret)
	}()

	return ret
}
