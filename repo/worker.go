package repo

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/nicolai86/github-rebase-bot/log"
	"github.com/nicolai86/github-rebase-bot/repo/internal/cmd"
)

// Worker manages a single branch for a repository
type Worker struct {
	cache  *Cache
	branch string
	prID   int
	dir    string
	Queue  chan chan Signal
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
	return strings.Contains(stdout, "is up to date"), nil
}

func (w *Worker) push() error {
	cmd := exec.Command("git", "push", "--set-upstream", "origin", w.branch, "-f")
	cmd.Dir = w.dir
	cmd.Env = os.Environ()
	return cmd.Run()
}

func (w *Worker) prepare() error {
	stdout, stderr, err := cmd.Pipeline([]*exec.Cmd{
		cmd.MustConfigure(exec.Command("git", "worktree", "add", w.dir, fmt.Sprintf("remotes/origin/%s", w.branch)), w.cache.inCacheDirectory()),
		cmd.MustConfigure(exec.Command("git", "checkout", w.branch), w.inCacheDirectory()),
	}).Run()
	log.PrintLinesPrefixed(w.branch, stdout)
	log.PrintLinesPrefixed(w.branch, stderr)
	return err
}

func (w *Worker) update() error {
	stdout, stderr, err := cmd.Pipeline([]*exec.Cmd{
		cmd.MustConfigure(exec.Command("git", "fetch", "origin", w.branch), w.inCacheDirectory()),
		cmd.MustConfigure(exec.Command("git", "reset", "--hard", fmt.Sprintf("origin/%s", w.branch)), w.inCacheDirectory()),
	}).Run()
	log.PrintLinesPrefixed(w.branch, stdout)
	log.PrintLinesPrefixed(w.branch, stderr)
	return err
}

func (w *Worker) run() {
	for {
		select {
		case ch := <-w.Queue:
			func(ch chan Signal) {
				rev, err := w.cache.update()
				if err != nil {
					log.Printf("failed to update master: %v", err)
					ch <- Signal{Error: err}
					close(ch)
					return
				}

				if err := w.update(); err != nil {
					log.Printf("failed to update worktree: %v", err)
					ch <- Signal{Error: err}
					close(ch)
					return
				}

				log.Printf("rebasing…")
				up2date, err := w.rebase()
				if err != nil {
					log.Printf("failed to rebase master: %v", err)
					ch <- Signal{Error: err}
					close(ch)
					return
				}

				if !up2date {
					if err := w.push(); err != nil {
						log.Printf("failed to push branch: %v", err)
						ch <- Signal{Error: err}
						close(ch)
						return
					}

					ch <- Signal{Error: nil, UpToDate: false}
					close(ch)
					return
				}

				// if master changed, re-try
				rev2, _ := w.cache.update()
				if rev != rev2 {
					w.Queue <- ch
					return
				}

				ch <- Signal{Error: err, UpToDate: true}
				close(ch)
			}(ch)
		case <-time.After(24 * time.Hour):
			log.Printf("shutdown")
			w.cache.Cleanup(w.prID)
			return
		}
	}
}
