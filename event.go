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

			// TODO check for PRs which could be affected by this merge (e.g. green and Ready to merge)

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
	rebaseQueue := verifyPullRequest(client.Issues, client.Repositories, mergeLabel, merge(
		prQueue,
		processIssuesEvent(client.PullRequests, issueQueue),
		processStatusEvent(client.PullRequests, statusQueue),
		processPullRequestReviewEvent(client, reviewQueue),
	))

	doneQueue := processMerge(client,
		processRebase(cache, rebaseQueue),
	)

	go func() {
		for pr := range doneQueue {
			fmt.Printf("merged PR #%d\n", *pr.Number)

			// re-evaluate all open PRs to kick off new rebase if necessary
			prs, _, err := client.PullRequests.List(
				context.Background(),
				owner,
				repository,
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

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		eventType := r.Header.Get("X-GitHub-Event")

		if eventType == "pull_request" {
			evt := new(github.PullRequestEvent)
			json.NewDecoder(r.Body).Decode(evt)

			prQueue <- evt.PullRequest

			if evt.PullRequest.GetState() == "closed" {
				cache.Cleanup(repo.StringGitWorktree(evt.PullRequest.Head.GetRef()))
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
