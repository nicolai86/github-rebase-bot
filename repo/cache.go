package repo

import (
	"fmt"
	"io/ioutil"
	"os/exec"
	"sync"

	"github.com/nicolai86/github-rebase-bot/cmd"
	"github.com/nicolai86/github-rebase-bot/log"
)

// Cache manages the checkout of a github repository as well as the master branch.
// Additionally a cache manages all workers connected to this particular checkout
type Cache struct {
	token  string
	owner  string
	repo   string
	branch string
	dir    string
	mu     sync.Mutex

	workers map[int]*Worker
}

func (c *Cache) inCacheDirectory() func(*exec.Cmd) {
	return func(cmd *exec.Cmd) {
		cmd.Dir = c.dir
	}
}

// New returns a new cache and starts a checkout in the background
func New(token, owner, repo, branch string) (*Cache, error) {
	dir, err := ioutil.TempDir("", fmt.Sprintf("%s-%s-master", owner, repo))
	if err != nil {
		return nil, err
	}

	cache := &Cache{
		token:   token,
		owner:   owner,
		repo:    repo,
		branch:  branch,
		dir:     dir,
		workers: make(map[int]*Worker),
	}

	return cache, nil
}

func (c *Cache) update() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	stdout, stderr, err := cmd.Pipeline([]*exec.Cmd{
		cmd.MustConfigure(exec.Command("git", "fetch", "--all"), c.inCacheDirectory()),
		cmd.MustConfigure(exec.Command("git", "reset", "--hard", "origin/master"), c.inCacheDirectory()),
	}).Run()
	log.PrintLinesPrefixed("master", stdout)
	log.PrintLinesPrefixed("master", stderr)
	if err != nil {
		log.Fatalf("Failed to update cache for master: %q", err)
	}
	return nil
}

func (c *Cache) remove(w *Worker) {
	delete(c.workers, w.prID)
}

// Prepare clones the given branch from github
func (c *Cache) Prepare() error {
	return exec.Command(
		"git",
		"clone",
		fmt.Sprintf("https://%s@github.com/%s/%s.git", c.token, c.owner, c.repo),
		"--branch",
		c.branch,
		c.dir,
	).Run()
}

// Worker manages workers for branches. By default a worker runs in its own
// goroutine and is re-used if the same branch is requested multiple times
func (c *Cache) Worker(branch string, pr int) (*Worker, error) {
	w, ok := c.workers[pr]
	if ok {
		return w, nil
	}

	dir, err := ioutil.TempDir("", fmt.Sprintf("%s-%s-%d", c.owner, c.repo, pr))
	if err != nil {
		return nil, err
	}
	w = &Worker{
		branch: branch,
		prID:   pr,
		dir:    dir,
		cache:  c,
		Queue:  make(chan chan error),
	}
	c.workers[pr] = w
	if err := w.prepare(); err != nil {
		log.Printf("Preparing worktree failed: %#v", err)
		return nil, err
	}
	go w.run()
	return w, nil
}
