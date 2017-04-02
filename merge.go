package main

import (
	"sync"

	"github.com/google/go-github/github"
)

func merge(cs ...<-chan *github.PullRequest) <-chan *github.PullRequest {
	var wg sync.WaitGroup
	out := make(chan *github.PullRequest)

	output := func(c <-chan *github.PullRequest) {
		for evt := range c {
			out <- evt
		}
		wg.Done()
	}
	wg.Add(len(cs))

	for _, c := range cs {
		go output(c)
	}

	go func() {
		wg.Wait()
		close(out)
	}()
	return out
}
