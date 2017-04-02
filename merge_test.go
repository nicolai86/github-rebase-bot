package main

import (
	"testing"

	"github.com/google/go-github/github"
)

func TestMerge(t *testing.T) {
	c1 := make(chan *github.PullRequest, 2)
	c2 := make(chan *github.PullRequest, 2)
	c3 := merge(c1, c2)

	c1 <- &github.PullRequest{Number: intVal(1)}
	c2 <- &github.PullRequest{Number: intVal(2)}
	c2 <- &github.PullRequest{Number: intVal(3)}

	close(c1)
	close(c2)

	<-c3
	<-c3
	<-c3

	if _, ok := (<-c3); ok {
		t.Error("Expected c3 to be closed, but isn't")
	}
}
