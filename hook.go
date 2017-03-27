package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/go-github/github"
)

func createHook(client *github.Client, publicDNS, owner, repo string) (*github.Hook, error) {
	hook, _, err := client.Repositories.CreateHook(context.Background(), owner, repo, &github.Hook{
		Name:   github.String("web"),
		Active: github.Bool(true),
		Config: map[string]interface{}{
			"url":          fmt.Sprintf("%s/events", publicDNS),
			"content_type": "json",
		},
		Events: []string{
			"pull_request",
			"status",
			"pull_request_review",
			"issues",
		},
	})
	return hook, err
}

func lookupHook(client *github.Client, publicDNS, owner, repo string) (*github.Hook, error) {
	hooks, _, err := client.Repositories.ListHooks(context.Background(), owner, repo, &github.ListOptions{})
	if err != nil {
		return nil, err
	}

	var h *github.Hook
	for _, hook := range hooks {
		if url, ok := hook.Config["url"].(string); ok {
			if strings.Contains(url, publicDNS) {
				h = hook
			}
		}
	}
	return h, nil
}

func registerHook(client *github.Client, publicDNS, owner, repo string) (*github.Hook, error) {
	hook, err := lookupHook(client, publicDNS, owner, repo)
	if err != nil {
		return nil, err
	}

	if hook == nil {
		hook, err = createHook(client, publicDNS, owner, repo)
		if err != nil {
			return nil, err
		}
	}
	return hook, nil
}
