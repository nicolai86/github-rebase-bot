package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/google/go-github/github"
	"github.com/nicolai86/github-rebase-bot/repo"
	"golang.org/x/oauth2"
)

var (
	token      string
	owner      string
	repository string

	cache *repo.Cache
)

func main() {
	var publicDNS string
	flag.StringVar(&token, "github-token", "", "auth token for GH")
	flag.StringVar(&owner, "owner", "", "github owner")
	flag.StringVar(&repository, "repo", "", "github repo (owned by owner)")
	flag.StringVar(&publicDNS, "public-dns", "", "publicly accessible dns endpoint for webhook push")
	flag.Parse()

	{
		c, err := repo.New(token, owner, repository)
		if err != nil {
			log.Fatal(err)
		}
		cache = c
	}

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(oauth2.NoContext, ts)

	client := github.NewClient(tc)

	user, _, err := client.Users.Get(context.Background(), "")
	if err != nil {
		log.Fatal(err)
	}
	username := *user.Login

	log.Printf("Bot started for user %s.", username)

	var h *github.Hook
	if publicDNS != "" {
		h, err = registerHook(client, publicDNS, owner, repository)
		if err != nil {
			log.Fatal(err)
		}
	}

	// On ^C, or SIGTERM handle exit.
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	signal.Notify(c, syscall.SIGTERM)
	go func() {
		for sig := range c {
			log.Printf("Received %s, exiting.", sig.String())
			if h != nil {
				client.Repositories.DeleteHook(context.Background(), owner, repository, *h.ID)
			}
			os.Exit(0)
		}
	}()

	http.HandleFunc("/events", prHandler(client))

	log.Println("Listening on :8081")
	http.ListenAndServe(":8081", nil)
}
