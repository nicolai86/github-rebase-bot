package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/google/go-github/github"
	"github.com/nicolai86/github-rebase-bot/repo"
	"golang.org/x/oauth2"
)

var (
	token      string
	owner      string
	repository string
	mergeLabel string
	mainline   string

	cache *repo.Cache
)

func main() {
	var publicDNS string
	flag.StringVar(&token, "github-token", "", "auth token for GH")
	if token == "" {
		token = os.Getenv("GITHUB_TOKEN")
	}
	var addr string
	flag.StringVar(&owner, "owner", "", "github owner")
	flag.StringVar(&repository, "repo", "", "github repo (owned by owner)")
	flag.StringVar(&publicDNS, "public-dns", "", "publicly accessible dns endpoint for webhook push")
	flag.StringVar(&mergeLabel, "merge-label", "", "which label is checked to kick off the merge process")
	flag.StringVar(&addr, "addr", "", "address to listen on")
	flag.StringVar(&mainline, "mainline", "master", "main branch")
	flag.Parse()

	if token == "" {
		log.Fatal("Missing github token.")
	}

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(oauth2.NoContext, ts)

	client := github.NewClient(tc)

	user, _, err := client.Users.Get(context.Background(), "")
	if err != nil {
		log.Fatalf("resolving github user failed: %v", err)
	}
	username := *user.Login

	log.Printf("Bot started for user %s.\n", username)
	log.Printf("Using %q as merge-label.\n", mergeLabel)

	if err := exec.Command("git", "config", "--global", "user.name", "rebase bot").Run(); err != nil {
		log.Fatal("git config --global user.name failed: %q", err)
	}
	if err := exec.Command("git", "config", "--global", "user.email", "rebase-bot@your.domain.com").Run(); err != nil {
		log.Fatal("git config --global user.email failed: %q", err)
	}

	{
		url := fmt.Sprintf("https://%s@github.com/%s/%s.git", token, owner, repository)
		c, err := repo.Prepare(url, mainline)
		if err != nil {
			log.Fatalf("prepare failed: %v", err)
		}
		cache = c
	}

	// On ^C, or SIGTERM handle exit.
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	signal.Notify(c, syscall.SIGTERM)

	mux := http.NewServeMux()
	mux.HandleFunc("/events", prHandler(client))
	srv := &http.Server{
		Addr:    addr,
		Handler: mux,
	}
	log.Printf("Listening on %q\n", addr)
	go func() {
		srv.ListenAndServe()
	}()

	var h *github.Hook
	if publicDNS != "" {
		h, err = registerHook(client, publicDNS, owner, repository)
		if err != nil {
			log.Fatal(err)
		}
	}

	sig := <-c
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	srv.Shutdown(ctx)
	cancel()
	log.Printf("Received %s, exiting.", sig.String())
	if h != nil {
		client.Repositories.DeleteHook(context.Background(), owner, repository, *h.ID)
	}
}

func createHook(client *github.Client, publicDNS, owner, repo string) (*github.Hook, error) {
	hook, _, err := client.Repositories.CreateHook(context.Background(), owner, repo, &github.Hook{
		Name:   github.String("web"),
		Active: github.Bool(true),
		Config: map[string]interface{}{
			"url":          fmt.Sprintf("%s/events", publicDNS),
			"content_type": "json",
		},
		Events: []string{"*"},
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
