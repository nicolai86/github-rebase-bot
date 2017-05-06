package repo

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/nicolai86/github-rebase-bot/log"
	"github.com/nicolai86/github-rebase-bot/repo/internal/cmd"
)

type GitCache interface {
	Update() (string, error)
	cacheDirectory() string
	Mainline() string
	Cleanup(GitWorktree) error
	inCacheDirectory() func(*exec.Cmd)
}

type GitWorker interface {
	prepare() (string, error)
	update(string) error
	rebase(string) (bool, error)
	push(string) error
	Branch() string
}

type Enqueuer interface {
	Enqueue(chan Signal)
}

// Worker manages a single branch for a repository
type Worker struct {
	cache  GitCache
	branch string
	queue  chan chan Signal
	stop   context.CancelFunc
}

func (w *Worker) Branch() string {
	return w.branch
}

func (w *Worker) Enqueue(c chan Signal) {
	w.queue <- c
}

func inDir(dir string) func(*exec.Cmd) {
	return func(cmd *exec.Cmd) {
		cmd.Dir = dir
	}
}

func (w *Worker) rebase(dir string) (bool, error) {
	stdout, stderr, err := cmd.Pipeline([]*exec.Cmd{
		cmd.MustConfigure(exec.Command("git", "rebase", fmt.Sprintf("origin/%s", w.cache.Mainline())), inDir(dir)),
	}).Run()
	log.PrintLinesPrefixed(w.branch, stdout)
	log.PrintLinesPrefixed(w.branch, stderr)
	if err != nil {
		return false, err
	}
	return strings.Contains(stdout, "is up to date"), nil
}

func (w *Worker) push(dir string) error {
	cmd := exec.Command("git", "push", "--set-upstream", "origin", w.branch, "-f")
	cmd.Dir = dir
	cmd.Env = os.Environ()
	return cmd.Run()
}

func (w *Worker) prepare() (string, error) {
	dir, err := ioutil.TempDir("", fmt.Sprintf("%s-%s", path.Base(w.cache.cacheDirectory()), path.Base(w.branch)))
	if err != nil {
		return "", err
	}

	if err := removeWorktreeBranch(w.cache.cacheDirectory(), w.branch); err != nil {
		return "", err
	}

	stdout, stderr, err := cmd.Pipeline([]*exec.Cmd{
		cmd.MustConfigure(exec.Command("git", "worktree", "add", dir, fmt.Sprintf("remotes/origin/%s", w.branch)), w.cache.inCacheDirectory()),
		cmd.MustConfigure(exec.Command("git", "checkout", w.branch), inDir(dir)),
	}).Run()
	log.PrintLinesPrefixed(w.branch, stdout)
	log.PrintLinesPrefixed(w.branch, stderr)
	return dir, err
}

func (w *Worker) update(dir string) error {
	stdout, stderr, err := cmd.Pipeline([]*exec.Cmd{
		cmd.MustConfigure(exec.Command("git", "fetch", "origin", w.branch), inDir(dir)),
		cmd.MustConfigure(exec.Command("git", "reset", "--hard", fmt.Sprintf("origin/%s", w.branch)), inDir(dir)),
	}).Run()
	log.PrintLinesPrefixed(w.branch, stdout)
	log.PrintLinesPrefixed(w.branch, stderr)
	return err
}
