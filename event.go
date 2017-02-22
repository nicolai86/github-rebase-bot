package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/google/go-github/github"
)

// TODO ensure same PRs are not merged in parallel: introduce a merge queue

func prHandler(client *github.Client) http.HandlerFunc {
	var issueQueue = make(chan *github.IssuesEvent)
	var prQueue = make(chan *github.PullRequest)
	var rebaseQueue = make(chan int)
	var mergeQueue = make(chan *github.PullRequest)
	var statusQueue = make(chan *github.StatusEvent)

	// process rebased, successful pull requests. merge if possible (tests green, mergeable)
	go func() {
		log.Printf("merge queue: started")

		for {
			pr := <-mergeQueue

			status, _, err := client.Repositories.GetCombinedStatus(owner, repo, *pr.Head.SHA, &github.ListOptions{})
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
				repo,
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
		log.Printf("rebase queue: started")

		for {
			prID := <-rebaseQueue
			pr, _, err := client.PullRequests.Get(owner, repo, prID)
			if err != nil {
				continue
			}

			// TODO do not clone all the time. clone once on startup to speed things up. every worker needs separate repo clone
			dir, err := clone(token, owner, repo, *pr.Head.Ref)
			if err != nil {
				continue
			}

			rebaseChanged, err := rebase(dir)
			if err != nil {
				// TODO add comment that PR can not be rebased/ pushed automatically
				continue
			}
			if rebaseChanged {
				if err := push(dir); err != nil {
					// TODO add comment that PR can not be rebased/ pushed automatically
					continue
				}
			}

			mergeQueue <- pr
		}
	}()

	// process pull request candidates; only LGTM'd pull requests proceed
	go func() {
		log.Printf("lgtm queue: started")
		for {
			pr := <-prQueue

			issue, _, err := client.Issues.Get(owner, repo, *pr.Number)
			if err != nil {
				continue
			}

			if len(issue.Labels) == 0 {
				client.Issues.AddLabelsToIssue(owner, repo, *pr.Number, []string{"WIP"})
				continue
			}

			isLGTM := false
			for _, label := range issue.Labels {
				isLGTM = isLGTM || *label.Name == "LGTM"
			}
			if !isLGTM {
				continue
			}

			rebaseQueue <- *pr.Number
		}
	}()

	// if an issue event references an active pull request put the pull request onto the merge queue
	go func() {
		for {
			evt := <-issueQueue

			pr, _, err := client.PullRequests.Get(owner, repo, *evt.Issue.Number)
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

			prs, _, err := client.PullRequests.List(owner, repo, &github.PullRequestListOptions{
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
