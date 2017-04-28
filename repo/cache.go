package repo

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/nicolai86/github-rebase-bot/log"
	"github.com/nicolai86/github-rebase-bot/repo/internal/cmd"
)

// Cache manages the checkout of a github repository as well as the master branch.
// Additionally a cache manages all workers connected to this particular checkout
type Cache struct {
	dir string
	mu  sync.Mutex

	workers map[string]*Worker
}

func (c *Cache) inCacheDirectory() func(*exec.Cmd) {
	return func(cmd *exec.Cmd) {
		cmd.Dir = c.dir
	}
}

func (c *Cache) Update() (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	stdout, stderr, err := cmd.Pipeline([]*exec.Cmd{
		cmd.MustConfigure(exec.Command("git", "fetch", "--all"), c.inCacheDirectory()),
		cmd.MustConfigure(exec.Command("git", "reset", "--hard", "origin/master"), c.inCacheDirectory()),
		cmd.MustConfigure(exec.Command("git", "rev-parse", "HEAD"), c.inCacheDirectory()),
	}).Run()
	log.PrintLinesPrefixed("master", stdout)
	log.PrintLinesPrefixed("master", stderr)
	if err != nil {
		log.Fatalf("Failed to update cache for master: %q", err)
	}

	lines := strings.Split(stdout, "\n")
	rev := lines[len(lines)-2]
	return rev, nil
}

func (c *Cache) remove(w *Worker) {
	delete(c.workers, w.branch)
}

// Prepare clones the given branch from github and returns a Cache
func Prepare(url, branch string) (*Cache, error) {
	dir, err := ioutil.TempDir("", branch)
	if err != nil {
		return nil, err
	}

	cmd := exec.Command(
		"git",
		"clone",
		url,
		"--branch",
		branch,
		dir,
	)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	if err := cmd.Run(); err != nil {
		return nil, err
	}

	return &Cache{
		dir:     dir,
		workers: make(map[string]*Worker),
	}, nil
}

type GitWorktree interface {
	Branch() string
}

type StringGitWorktree string

func (w StringGitWorktree) Branch() string {
	return string(w)
}

func removeWorktreeBranch(dir, branch string) error {
	path := ""

	stdout, _, err := cmd.Pipeline([]*exec.Cmd{
		cmd.MustConfigure(exec.Command("git", "worktree", "prune"), inDir(dir)),
		cmd.MustConfigure(exec.Command("git", "worktree", "list"), inDir(dir)),
	}).Run()
	if err != nil {
		return err
	}

	lines := strings.Split(stdout, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasSuffix(line, fmt.Sprintf("[%s]", branch)) || strings.HasSuffix(line, fmt.Sprintf("[remotes/origin/%s]", branch)) {
			parts := strings.Split(line, " ")
			path = parts[0]
			break
		}
	}

	if path == "" {
		return nil
	}

	_, _, err = cmd.Pipeline([]*exec.Cmd{
		exec.Command("rm", "-fr", path),
		cmd.MustConfigure(exec.Command("git", "worktree", "prune"), inDir(dir)),
	}).Run()
	return err
}

// Cleanup removes a branch
// called when a pr is closed
func (c *Cache) Cleanup(v GitWorktree) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	w, ok := c.workers[v.Branch()]
	if !ok {
		return nil
	}

	removeWorktreeBranch(c.cacheDirectory(), v.Branch())

	stdout, stderr, err := cmd.Pipeline([]*exec.Cmd{
		cmd.MustConfigure(exec.Command("git", "worktree", "prune"), c.inCacheDirectory()),
	}).Run()
	log.PrintLinesPrefixed(w.branch, stdout)
	log.PrintLinesPrefixed(w.branch, stderr)
	if err != nil {
		log.Printf("worktree cleanup failed: %q", err)
	}
	w.stop()
	delete(c.workers, v.Branch())
	return nil
}

func (c *Cache) cacheDirectory() string {
	return c.dir
}

// Worker manages workers for branches. By default a worker runs in its own
// goroutine and is re-used if the same branch is requested multiple times
func (c *Cache) Worker(branch string) (Enqueuer, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	w, ok := c.workers[branch]
	if ok {
		return w, nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	w = &Worker{
		branch: branch,
		cache:  c,
		queue:  make(chan chan Signal),
		stop:   cancel,
	}
	c.workers[branch] = w

	rebaser := branchRebaser{
		w:     w,
		cache: c,
		queue: w.queue,
		ctx:   ctx,
	}
	go rebaser.run()
	return w, nil
}
