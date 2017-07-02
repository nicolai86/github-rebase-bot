package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/google/go-github/github"
	"github.com/nicolai86/github-rebase-bot/processors"
	"github.com/nicolai86/github-rebase-bot/repo"
)

type statusEventBroadcaster struct {
	listeners []chan<- *github.StatusEvent
}

func (b *statusEventBroadcaster) Listen(in <-chan *github.StatusEvent) {
	for evt := range in {
		for _, l := range b.listeners {
			l <- evt
		}
	}

	for _, l := range b.listeners {
		close(l)
	}
}

func prHandler(r repository, client *github.Client) http.HandlerFunc {
	issueQueue := make(chan *github.IssuesEvent, 100)
	prQueue := make(chan *github.PullRequest, 100)
	reviewQueue := make(chan *github.PullRequestReviewEvent, 100)
	pushEventQueue := make(chan *github.PushEvent, 100)
	statusEventQueue := make(chan *github.StatusEvent, 100)
	statusPRQueue := make(chan *github.StatusEvent, 100)
	mainlineStatusEventQueue := make(chan *github.StatusEvent, 100)

	statusBroadcaster := statusEventBroadcaster{
		listeners: []chan<- *github.StatusEvent{
			statusPRQueue,
			mainlineStatusEventQueue,
		},
	}
	go statusBroadcaster.Listen(statusEventQueue)

	// rebase queue contains pull requests which are:
	//  - open
	//  - green
	//  - marked with mergeLabel
	//  - mergeable
	rebaseQueue := verifyPullRequest(client.Issues, client.Repositories, mergeLabel, merge(
		prQueue,
		processors.MainlineStatusEvent(r.Repository, client.PullRequests, mainlineStatusEventQueue),
		processors.IssuesEvent(client.PullRequests, issueQueue),
		processors.StatusEvent(client.PullRequests, statusPRQueue),
		processors.PushEvent(r.Repository, client.PullRequests, pushEventQueue),
		processors.PullRequestReviewEvent(client, reviewQueue),
	))

	handleRebase := func(input <-chan processors.RebaseResult) <-chan *github.PullRequest {
		ret := make(chan *github.PullRequest)
		go func() {
			for res := range input {
				// allow PRs which rebased without error to pass through
				if res.Error == nil {
					ret <- res.PR
					continue
				}

				// retry PRs where mainline changed during the rebase
				if res.Error == processors.ErrMainlineChanged {
					prQueue <- res.PR
					continue
				}

				log.Printf("filtering PR #%d on %s because of %v", res.PR.GetNumber(), r.Name, res.Error)
			}
		}()
		return ret
	}

	doneQueue := processors.Merge(client,
		handleRebase(processors.Rebase(r.Repository, rebaseQueue)),
	)

	go func() {
		for pr := range doneQueue {
			fmt.Printf("merged PR #%d\n", *pr.Number)

			// re-evaluate all open PRs to kick off new rebase if necessary
			prs, _, err := client.PullRequests.List(
				context.Background(),
				pr.Base.User.GetLogin(),
				pr.Base.Repo.GetName(),
				&github.PullRequestListOptions{
					State: "open",
				})
			if err != nil {
				continue
			}
			for _, pr := range prs {
				prQueue <- pr
			}
		}
	}()

	// evaluate all open PRs on startup to kick off new rebase if necessary
	prs, _, err := client.PullRequests.List(
		context.Background(),
		r.Owner,
		r.Name,
		&github.PullRequestListOptions{
			State: "open",
		})
	if err != nil {
		log.Printf("failed to populate open PRs on startup: %v", err)
	} else {
		for _, pr := range prs {
			prQueue <- pr
		}
	}

	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		eventType := req.Header.Get("X-GitHub-Event")

		if eventType == "pull_request" {
			evt := new(github.PullRequestEvent)
			json.NewDecoder(req.Body).Decode(evt)

			prQueue <- evt.PullRequest

			if evt.PullRequest.GetState() == "closed" {
				cache := repos.Find(evt.PullRequest.Base.User.GetLogin(), evt.PullRequest.Base.Repo.GetName()).Repository.Cache
				cache.Cleanup(repo.StringGitWorktree(evt.PullRequest.Head.GetRef()))
			}
		} else if eventType == "pull_request_review" {
			evt := new(github.PullRequestReviewEvent)
			json.NewDecoder(req.Body).Decode(evt)

			reviewQueue <- evt
		} else if eventType == "issues" {
			evt := new(github.IssuesEvent)
			json.NewDecoder(req.Body).Decode(evt)

			issueQueue <- evt
		} else if eventType == "status" {
			evt := new(github.StatusEvent)
			json.NewDecoder(req.Body).Decode(evt)

			statusEventQueue <- evt
		} else if eventType == "push" {
			evt := new(github.PushEvent)
			json.NewDecoder(req.Body).Decode(evt)

			pushEventQueue <- evt
		} else {
			log.Printf("%s/%s: Event %s not supported yet.\n", r.Owner, r.Name, eventType)
		}
	})
}
