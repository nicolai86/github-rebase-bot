package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/google/go-github/github"
	"github.com/nicolai86/github-rebase-bot/repo"
)

func processPullRequestReviewEvent(client *github.Client, input <-chan *github.PullRequestReviewEvent) <-chan *github.PullRequest {
	ret := make(chan *github.PullRequest)
	go func() {
		for evt := range input {
			ret <- evt.PullRequest
		}
		close(ret)
	}()
	return ret
}

func processRebase(client *github.Client, input <-chan *github.PullRequest) <-chan *github.PullRequest {
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
			c := make(chan repo.Signal)
			w.Queue <- c
			go func(pr *github.PullRequest) {
				sig := <-c
				if sig.UpToDate && sig.Error == nil {
					ret <- pr
				}
			}(pr)
		}
		close(ret)
	}()
	return ret
}

func processMerge(client *github.Client, input <-chan *github.PullRequest) <-chan *github.PullRequest {
	ret := make(chan *github.PullRequest)
	go func() {
		for pr := range input {
			if _, _, err := client.PullRequests.Merge(
				context.Background(),
				owner,
				repository,
				pr.GetNumber(),
				"merge-bot merged",
				&github.PullRequestOptions{
					MergeMethod: "merge",
				}); err != nil {
				continue
			}

			if _, err := client.Git.DeleteRef(
				context.Background(),
				owner,
				repository,
				fmt.Sprintf("heads/%s", *pr.Head.Ref),
			); err != nil {
				fmt.Printf("Failed deleting branch: %q\n", err)
			}

			ret <- pr
		}
		close(ret)
	}()
	return ret
}

func prHandler(client *github.Client) http.HandlerFunc {
	issueQueue := make(chan *github.IssuesEvent, 100)
	prQueue := make(chan *github.PullRequest, 100)
	reviewQueue := make(chan *github.PullRequestReviewEvent, 100)
	statusQueue := make(chan *github.StatusEvent, 100)

	// rebase queue contains pull requests which are:
	//  - open
	//  - green
	//  - marked with mergeLabel
	//  - mergeable
	rebaseQueue := processPullRequest(client.Issues, client.Repositories, mergeLabel, merge(
		prQueue,
		processIssuesEvent(client.PullRequests, issueQueue),
		processStatusEvent(client.PullRequests, statusQueue),
		processPullRequestReviewEvent(client, reviewQueue),
	))

	doneQueue := processMerge(client,
		processRebase(client, rebaseQueue),
	)

	go func() {
		for pr := range doneQueue {
			fmt.Printf("merged PR #%d\n", *pr.Number)
		}
	}()

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		eventType := r.Header.Get("X-GitHub-Event")

		if eventType == "pull_request" {
			evt := new(github.PullRequestEvent)
			json.NewDecoder(r.Body).Decode(evt)

			prQueue <- evt.PullRequest

			if evt.PullRequest.GetState() == "closed" {
				cache.Cleanup(*evt.PullRequest.Number)
			}
		} else if eventType == "pull_request_review" {
			evt := new(github.PullRequestReviewEvent)
			json.NewDecoder(r.Body).Decode(evt)

			reviewQueue <- evt
		} else if eventType == "issues" {
			evt := new(github.IssuesEvent)
			json.NewDecoder(r.Body).Decode(evt)

			issueQueue <- evt
		} else if eventType == "status" {
			evt := new(github.StatusEvent)
			json.NewDecoder(r.Body).Decode(evt)

			statusQueue <- evt
		} else {
			log.Printf("Event %s not supported yet.\n", eventType)
		}
	})
}
