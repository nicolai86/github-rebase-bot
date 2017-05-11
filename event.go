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
				pr.Base.User.GetLogin(),
				pr.Base.Repo.GetName(),
				pr.GetNumber(),
				"merge-bot merged",
				&github.PullRequestOptions{
					MergeMethod: "merge",
				}); err != nil {
				continue
			}

			if _, err := client.Git.DeleteRef(
				context.Background(),
				pr.Base.User.GetLogin(),
				pr.Base.Repo.GetName(),
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
		processMainlineStatusEvent(repos, client.PullRequests, mainlineStatusEventQueue),
		processIssuesEvent(client.PullRequests, issueQueue),
		processStatusEvent(client.PullRequests, statusPRQueue),
		processPushEvent(repos, client.PullRequests, pushEventQueue),
		processPullRequestReviewEvent(client, reviewQueue),
	))

	doneQueue := processMerge(client,
		processRebase(repos, rebaseQueue),
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

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		eventType := r.Header.Get("X-GitHub-Event")

		if eventType == "pull_request" {
			evt := new(github.PullRequestEvent)
			json.NewDecoder(r.Body).Decode(evt)

			prQueue <- evt.PullRequest

			if evt.PullRequest.GetState() == "closed" {
				cache := repos.Find(evt.PullRequest.Base.User.GetLogin(), evt.PullRequest.Base.Repo.GetName()).cache
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

			statusEventQueue <- evt
		} else if eventType == "push" {
			evt := new(github.PushEvent)
			json.NewDecoder(r.Body).Decode(evt)

			pushEventQueue <- evt
		} else {
			log.Printf("Event %s not supported yet.\n", eventType)
		}
	})
}
