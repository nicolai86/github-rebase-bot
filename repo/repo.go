package repo

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/nicolai86/github-rebase-bot/cmd"
)

type Cache struct {
	token string
	owner string
	repo  string
	dir   string
	mu    sync.Mutex

	populate chan struct{}
	workers  map[int]*Worker
}

func (c *Cache) inCacheDirectory() func(*exec.Cmd) {
	return func(cmd *exec.Cmd) {
		cmd.Dir = c.dir
	}
}

type Worker struct {
	cache  *Cache
	branch string
	prID   int
	dir    string
	Queue  chan chan error
}

func (w *Worker) inCacheDirectory() func(*exec.Cmd) {
	return func(cmd *exec.Cmd) {
		cmd.Dir = w.dir
	}
}

func (w *Worker) rebase() (bool, error) {
	var stdout bytes.Buffer
	cmd := exec.Command("git", "rebase", "origin/master")
	cmd.Dir = w.dir
	cmd.Stdout = &stdout
	cmd.Env = os.Environ()
	err := cmd.Run()
	if err != nil {
		return false, err
	}
	up2date := strings.Contains(string(stdout.Bytes()), "is up to date")
	return !up2date, nil
}

func (w *Worker) push() error {
	cmd := exec.Command("git", "push", "--set-upstream", "origin", w.branch, "-f")
	cmd.Dir = w.dir
	cmd.Env = os.Environ()
	return cmd.Run()
}

func (w *Worker) prepare() error {
	return cmd.Pipeline([]*exec.Cmd{
		cmd.MustConfigure(exec.Command("git", "fetch", "origin", w.branch), w.cache.inCacheDirectory()),
		cmd.MustConfigure(exec.Command("git", "worktree", "add", w.dir, fmt.Sprintf("remotes/origin/%s", w.branch)), w.cache.inCacheDirectory()),
		cmd.MustConfigure(exec.Command("git", "checkout", "-b", w.branch), w.inCacheDirectory()),
	}).Run()
}

func (w *Worker) cleanup() {
	p := cmd.Pipeline([]*exec.Cmd{
		cmd.MustConfigure(exec.Command("git", "worktree", "unlock", "-b", w.branch), w.cache.inCacheDirectory()),
		exec.Command("rm", "-fr", w.dir),
		cmd.MustConfigure(exec.Command("git", "worktree", "prune"), w.cache.inCacheDirectory()),
	})
	if err := p.Run(); err != nil {
		log.Fatal("worktree cleanup failed: %q", err)
	}
}

func (w *Worker) run() {
	for {
		select {
		case ch := <-w.Queue:
			w.prepare()
			if err := w.cache.update(); err != nil {
				log.Printf("failed to update master: %v", err)
				w.cleanup()
				ch <- err
				close(ch)
				continue
			}
			log.Printf("rebasing…")
			changed, err := w.rebase()
			if err != nil {
				log.Printf("failed to rebase master: %v", err)
				w.cleanup()
				ch <- err
				close(ch)
				continue
			}
			if changed {
				if err := w.push(); err != nil {
					log.Printf("failed to push branch: %v", err)
					w.cleanup()
					ch <- err
					continue
				}
				w.cleanup()
				close(ch)
			} else {
				w.cleanup()
				close(ch)
				log.Printf("up2date")
			}
		case <-time.After(24 * 7 * time.Hour):
			log.Printf("shutdown")
			w.cache.remove(w)
			return
		}
	}
}

func New(token, owner, repo string) (*Cache, error) {
	dir, err := ioutil.TempDir("", fmt.Sprintf("%s-%s-master", owner, repo))
	if err != nil {
		return nil, err
	}

	cache := &Cache{
		token:    token,
		owner:    owner,
		repo:     repo,
		dir:      dir,
		populate: make(chan struct{}),
		workers:  make(map[int]*Worker),
	}

	go func() {
		log.Printf("master cache: %s", cache.dir)
		{
			cmd := exec.Command("git", "clone",
				fmt.Sprintf("https://%s@github.com/%s/%s.git", cache.token, cache.owner, cache.repo),
				"--branch", "master",
				cache.dir,
			)
			cmd.Dir = cache.dir
			cmd.Env = os.Environ()
			if err := cmd.Run(); err != nil {
				log.Fatalf("populate for %q failed: %q", fmt.Sprintf("https://github.com/%s/%s.git", cache.owner, cache.repo), err)
			}
		}
		{
			cmd := exec.Command("git", "config", "--global", "user.email", "rebase-bot@your.domain.com")
			cmd.Dir = cache.dir
			cmd.Env = os.Environ()
			if err := cmd.Run(); err != nil {
				log.Fatalf("failed to configure global user")
			}
		}
		{
			cmd := exec.Command("git", "config", "--global", "user.name", "rebase bot")
			cmd.Dir = cache.dir
			cmd.Env = os.Environ()
			if err := cmd.Run(); err != nil {
				log.Fatalf("failed to configure global user")
			}
		}
		close(cache.populate)
	}()
	return cache, nil
}

func (c *Cache) update() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	cmd := exec.Command("git", "fetch", "--all")
	// cmd := exec.Command("git", "remote", "update")
	cmd.Dir = c.dir
	cmd.Env = os.Environ()
	if err := cmd.Run(); err != nil {
		log.Printf("git fetch --all failed")
		return err
	}

	cmd = exec.Command("git", "reset", "--hard", "origin/master")
	cmd.Dir = c.dir
	cmd.Env = os.Environ()
	if err := cmd.Run(); err != nil {
		log.Printf("git reset --hard origin/master failed")
		return err
	}
	return nil
}

func (c *Cache) remove(w *Worker) {
	delete(c.workers, w.prID)
}

func (c *Cache) Worker(branch string, pr int) (*Worker, error) {
	<-c.Wait()

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

func (c *Cache) Wait() <-chan struct{} {
	return c.populate
}
