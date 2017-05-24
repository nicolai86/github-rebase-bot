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
	"github.com/nicolai86/github-rebase-bot/processors"
	"github.com/nicolai86/github-rebase-bot/repo"
	"golang.org/x/oauth2"
)

var (
	token      string
	repos      repositories
	mergeLabel string
)

type repositories []repository

func (rs repositories) Find(owner, name string) *repository {
	for i := range rs {
		if rs[i].Owner == owner && rs[i].Name == name {
			return &rs[i]
		}
	}
	return nil
}

func (hps *repositories) String() string {
	return fmt.Sprint(*hps)
}

func (hps *repositories) Set(str string) error {
	for _, hp := range strings.Split(str, ",") {
		var h repository
		if err := h.Set(hp); err != nil {
			return err
		}
		*hps = append(*hps, h)
	}
	return nil
}

type repository struct {
	processors.Repository
	hook *github.Hook
}

func (h *repository) String() string {
	return fmt.Sprintf("%s/%s#%s", h.Owner, h.Name, h.Mainline)
}

func (h *repository) Set(str string) error {
	var parts = strings.Split(str, "/")
	if len(parts) != 2 {
		return fmt.Errorf("Invalid repository %q. Must be owner/name", str)
	}
	h.Owner = parts[0]
	parts = strings.Split(parts[1], "#")
	h.Name = parts[0]
	if len(parts) == 2 {
		h.Mainline = parts[1]
	}
	if h.Mainline == "" {
		h.Mainline = "master"
	}
	return nil
}

func main() {
	var publicDNS string
	flag.StringVar(&token, "github-token", "", "auth token for GH")
	if token == "" {
		token = os.Getenv("GITHUB_TOKEN")
	}
	var addr string
	flag.Var(&repos, "repos", "github repos (owner/repo separated by commas)")
	flag.StringVar(&publicDNS, "public-dns", "", "publicly accessible dns endpoint for webhook push")
	flag.StringVar(&mergeLabel, "merge-label", "", "which label is checked to kick off the merge process")
	flag.StringVar(&addr, "addr", "", "address to listen on")
	flag.Parse()

	if token == "" {
		log.Fatal("Missing github token.")
	}

	if len(repos) == 0 {
		log.Fatal("Missing repositories.")
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

	for i, r := range repos {
		url := fmt.Sprintf("https://%s@github.com/%s/%s.git", token, r.Owner, r.Name)
		c, err := repo.Prepare(url, r.Mainline)
		if err != nil {
			log.Fatalf("prepare failed: %v", err)
		}
		repos[i].Cache = c
	}

	// On ^C, or SIGTERM handle exit.
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	signal.Notify(c, syscall.SIGTERM)

	mux := http.NewServeMux()
	for _, repo := range repos {
		mux.HandleFunc(fmt.Sprintf("/events/%s/%s", repo.Owner, repo.Name), prHandler(repo, client))
	}
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
		for i, repo := range repos {
			h, err = registerHook(client, publicDNS, repo.Owner, repo.Name)
			if err != nil {
				log.Fatal(err)
			}
			repos[i].hook = h
		}
	}

	sig := <-c
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	srv.Shutdown(ctx)
	cancel()
	log.Printf("Received %s, exiting.", sig.String())
	if h != nil {
		for _, repo := range repos {
			client.Repositories.DeleteHook(context.Background(), repo.Owner, repo.Name, *repo.hook.ID)
		}
	}
}

func createHook(client *github.Client, publicDNS, owner, repo, hookTarget string) (*github.Hook, error) {
	hook, _, err := client.Repositories.CreateHook(context.Background(), owner, repo, &github.Hook{
		Name:   github.String("web"),
		Active: github.Bool(true),
		Config: map[string]interface{}{
			"url":          hookTarget,
			"content_type": "json",
		},
		Events: []string{"*"},
	})
	return hook, err
}

func lookupHook(client *github.Client, owner, repo, hookTarget string) (*github.Hook, error) {
	hooks, _, err := client.Repositories.ListHooks(context.Background(), owner, repo, &github.ListOptions{})
	if err != nil {
		return nil, err
	}

	var h *github.Hook
	for _, hook := range hooks {
		if url, ok := hook.Config["url"].(string); ok {
			if strings.Contains(url, hookTarget) {
				h = hook
				break
			}
		}
	}
	return h, nil
}

func registerHook(client *github.Client, publicDNS, owner, repo string) (*github.Hook, error) {
	hookTarget := fmt.Sprintf("%s/events/%s/%s", publicDNS, owner, repo)
	hook, err := lookupHook(client, owner, repo, hookTarget)
	if err != nil {
		return nil, err
	}

	if hook == nil {
		hook, err = createHook(client, publicDNS, owner, repo, hookTarget)
		if err != nil {
			return nil, err
		}
	}
	return hook, nil
}
