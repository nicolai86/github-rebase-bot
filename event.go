package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/google/go-github/github"
)

func prHandler(client *github.Client) http.HandlerFunc {
	var mergeQueue = make(chan *github.PullRequest)
	var rebaseQueue = make(chan int, 100)
	var issueQueue = make(chan *github.IssuesEvent, 100)
	var prQueue = make(chan *github.PullRequest, 100)
	var statusQueue = make(chan *github.StatusEvent, 100)

	// process rebased, successful pull requests. merge if possible (tests green, mergeable)
	go func() {
		log.Printf("merge queue: started")

		for {
			pr := <-mergeQueue

			status, _, err := client.Repositories.GetCombinedStatus(owner, repository, *pr.Head.SHA, &github.ListOptions{})
			if err != nil {
				continue
			}

			log.Printf("status for %q (%q): %q\n", *pr.Head.Ref, *pr.Head.SHA, *status.State)
			if *status.State != "success" {
				continue
			}

			if !*pr.Mergeable {
				continue
			}

			result, _, err := client.PullRequests.Merge(
				owner,
				repository,
				*pr.Number,
				"merge-bot merged",
				&github.PullRequestOptions{
					MergeMethod: "merge",
				})
			if err != nil {
				continue
			}

			fmt.Printf("merged: %v\n", result)
		}
	}()

	// process LGTM'd pull requests: rebase if necessary. can run in parallel. Can run in parallel
	go func() {
		<-cache.Wait()
		log.Printf("rebase queue: started")

		for {
			prID := <-rebaseQueue
			log.Printf("processing rebase PR #%d", prID)
			pr, _, err := client.PullRequests.Get(owner, repository, prID)
			if err != nil {
				log.Printf("Failed to fetch PR: %v", err)
				continue
			}

			w, err := cache.Worker(*pr.Head.Ref, prID)
			if err != nil {
				log.Printf("Failed to get worker: %v", err)
				continue
			}
			c := make(chan error)
			w.Queue <- c
			go func(pr *github.PullRequest) {
				err := <-c
				if err == nil {
					mergeQueue <- pr
				}
			}(pr)
		}
	}()

	// process pull request candidates; only LGTM'd pull requests proceed
	go func() {
		log.Printf("lgtm queue: started")
		for {
			pr := <-prQueue
			log.Printf("processing PR #%d", *pr.Number)
			if *pr.State != "open" {
				log.Printf("PR is not open")
				continue
			}

			issue, _, err := client.Issues.Get(owner, repository, *pr.Number)
			if err != nil {
				log.Printf("unable to fetch issues: %v", err)
				continue
			}

			if len(issue.Labels) == 0 {
				client.Issues.AddLabelsToIssue(owner, repository, *pr.Number, []string{"WIP"})
				log.Printf("Added WIP label")
				continue
			}

			isLGTM := false
			for _, label := range issue.Labels {
				isLGTM = isLGTM || *label.Name == "LGTM"
			}
			if !isLGTM {
				log.Printf("Not LGTM")
				continue
			}

			rebaseQueue <- *pr.Number
		}
	}()

	// if an issue event references an active pull request put the pull request onto the merge queue
	go func() {
		for {
			evt := <-issueQueue

			pr, _, err := client.PullRequests.Get(owner, repository, *evt.Issue.Number)
			if err != nil {
				continue
			}

			prQueue <- pr
		}
	}()

	// if a status event references an active pull request put the pull request onto the merge queue
	go func() {
		for {
			evt := <-statusQueue

			prs, _, err := client.PullRequests.List(owner, repository, &github.PullRequestListOptions{
				State: "open",
			})
			if err != nil {
				log.Printf("failed to list PRs: %#v", err)
				continue
			}
			var pr *github.PullRequest
			for _, branch := range evt.Branches {
				for _, p := range prs {
					if *p.Head.Ref == *branch.Name {
						pr = p
						break
					}
				}
				if pr != nil {
					break
				}
			}

			if pr != nil {
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
		} else if eventType == "pull_request_review" {
			evt := new(github.PullRequestReviewEvent)
			json.NewDecoder(r.Body).Decode(evt)

			prQueue <- evt.PullRequest
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
