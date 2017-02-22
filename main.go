package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"

	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
)

func clone(accessToken, owner, repo, branch string) (string, error) {
	dir, err := ioutil.TempDir("", fmt.Sprintf("gh-%s-%s", owner, repo))
	if err != nil {
		return "", err
	}

	cmds := [][]string{
		[]string{"git",
			"clone",
			fmt.Sprintf("https://%s@github.com/%s/%s.git", accessToken, owner, repo),
			"--branch", branch,
			// "--depth", "1",
			dir,
		},
	}

	var output bytes.Buffer
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		cmd.Stdout = &output
		cmd.Env = os.Environ()
		if err := cmd.Run(); err != nil {
			return "", err
		}
		log.Printf("%v: %v", args, string(output.Bytes()))
	}

	return dir, nil
}

func rebase(dir string) (bool, error) {
	cmds := [][]string{
		[]string{"git", "remote", "update"},
		[]string{"git", "rebase", "origin/master"},
	}

	var output bytes.Buffer
	for _, args := range cmds {
		output.Reset()

		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		cmd.Env = os.Environ()
		cmd.Stdout = &output
		if err := cmd.Run(); err != nil {
			return false, err
		}

		log.Printf("%v: %v", args, string(output.Bytes()))
	}

	if strings.Contains(string(output.Bytes()), "is up to date") {
		return false, nil
	}

	return true, nil
}

func push(dir string) error {
	cmds := [][]string{
		[]string{"git", "push", "-f"},
	}

	var output bytes.Buffer
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		cmd.Env = os.Environ()
		cmd.Stdout = &output
		if err := cmd.Run(); err != nil {
			return err
		}
		log.Printf("%v: %v", args, string(output.Bytes()))
	}

	return nil
}

var (
	token string
	owner string
	repo  string
)

func main() {
	var publicDNS string
	flag.StringVar(&token, "github-token", "", "auth token for GH")
	flag.StringVar(&owner, "owner", "", "github owner")
	flag.StringVar(&repo, "repo", "", "github repo (owned by owner)")
	flag.StringVar(&publicDNS, "public-dns", "", "publicly accessible dns endpoint for webhook push")
	flag.Parse()

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(oauth2.NoContext, ts)

	client := github.NewClient(tc)

	user, _, err := client.Users.Get("")
	if err != nil {
		log.Fatal(err)
	}
	username := *user.Login

	log.Printf("Bot started for user %s.", username)

	var h *github.Hook
	if publicDNS != "" {
		h, err = registerHook(client, publicDNS, owner, repo)
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
				client.Repositories.DeleteHook(owner, repo, *h.ID)
			}
			os.Exit(0)
		}
	}()

	http.HandleFunc("/events", prHandler(client))

	log.Println("Listening on :8081")
	http.ListenAndServe(":8081", nil)
}
