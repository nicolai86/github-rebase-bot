package repo

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/nicolai86/github-rebase-bot/cmd"
	"github.com/nicolai86/github-rebase-bot/log"
)

// Worker manages a single branch for a repository
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
	stdout, stderr, err := cmd.Pipeline([]*exec.Cmd{
		cmd.MustConfigure(exec.Command("git", "rebase", "origin/master"), w.inCacheDirectory()),
	}).Run()
	log.PrintLinesPrefixed(w.branch, stdout)
	log.PrintLinesPrefixed(w.branch, stderr)
	if err != nil {
		return false, err
	}
	up2date := strings.Contains(stdout, "is up to date")
	return !up2date, nil
}

func (w *Worker) push() error {
	cmd := exec.Command("git", "push", "--set-upstream", "origin", w.branch, "-f")
	cmd.Dir = w.dir
	cmd.Env = os.Environ()
	return cmd.Run()
}

func (w *Worker) prepare() error {
	stdout, stderr, err := cmd.Pipeline([]*exec.Cmd{
		cmd.MustConfigure(exec.Command("git", "fetch", "origin", w.branch), w.cache.inCacheDirectory()),
		cmd.MustConfigure(exec.Command("git", "worktree", "add", w.dir, fmt.Sprintf("remotes/origin/%s", w.branch)), w.cache.inCacheDirectory()),
		cmd.MustConfigure(exec.Command("git", "checkout", w.branch), w.inCacheDirectory()),
	}).Run()
	log.PrintLinesPrefixed(w.branch, stdout)
	log.PrintLinesPrefixed(w.branch, stderr)
	return err
}

func (w *Worker) cleanup() {
	stdout, stderr, err := cmd.Pipeline([]*exec.Cmd{
		exec.Command("rm", "-fr", w.dir),
		cmd.MustConfigure(exec.Command("git", "worktree", "prune"), w.cache.inCacheDirectory()),
	}).Run()
	log.PrintLinesPrefixed(w.branch, stdout)
	log.PrintLinesPrefixed(w.branch, stderr)
	if err != nil {
		log.Fatalf("worktree cleanup failed: %q", err)
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
