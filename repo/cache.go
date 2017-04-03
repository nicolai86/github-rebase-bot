package repo

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
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

	workers map[int]*Worker
}

func (c *Cache) inCacheDirectory() func(*exec.Cmd) {
	return func(cmd *exec.Cmd) {
		cmd.Dir = c.dir
	}
}

func (c *Cache) update() (string, error) {
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
	delete(c.workers, w.prID)
}

// Prepare clones the given branch from github and returns a Cache
func Prepare(token, owner, repo, branch string) (*Cache, error) {
	dir, err := ioutil.TempDir("", fmt.Sprintf("%s-%s-master", owner, repo))
	if err != nil {
		return nil, err
	}

	cmd := exec.Command(
		"git",
		"clone",
		fmt.Sprintf("https://%s@github.com/%s/%s.git", token, owner, repo),
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
		workers: make(map[int]*Worker),
	}, nil
}

func (c *Cache) Cleanup(id int) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	w, ok := c.workers[id]
	if !ok {
		return nil
	}
	stdout, stderr, err := cmd.Pipeline([]*exec.Cmd{
		exec.Command("rm", "-fr", w.dir),
		cmd.MustConfigure(exec.Command("git", "worktree", "prune"), c.inCacheDirectory()),
	}).Run()
	log.PrintLinesPrefixed(w.branch, stdout)
	log.PrintLinesPrefixed(w.branch, stderr)
	if err != nil {
		log.Printf("worktree cleanup failed: %q", err)
	}
	delete(c.workers, id)
	return nil
}

// Worker manages workers for branches. By default a worker runs in its own
// goroutine and is re-used if the same branch is requested multiple times
func (c *Cache) Worker(branch string, id int) (Enqueuer, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	w, ok := c.workers[id]
	if ok {
		return w, nil
	}

	dir, err := ioutil.TempDir("", fmt.Sprintf("%s-%d", path.Base(c.dir), id))
	if err != nil {
		return nil, err
	}
	w = &Worker{
		branch: branch,
		prID:   id,
		dir:    dir,
		cache:  c,
		queue:  make(chan chan Signal),
	}
	c.workers[id] = w
	if err := w.prepare(); err != nil {
		log.Printf("Preparing worktree failed: %#v", err)
		return nil, err
	}
	go w.run()
	return w, nil
}
