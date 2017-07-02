package main

import (
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
