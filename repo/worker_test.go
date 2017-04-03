package repo

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
	"testing"
)

func expandSHA(dir, shortSHA string) (string, error) {
	cmd := exec.Command("git", "rev-parse", shortSHA)
	cmd.Dir = dir
	var b bytes.Buffer
	cmd.Stdout = &b
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return strings.TrimSpace(b.String()), nil
}

func latestSHA(dir, branch string) (string, error) {
	cmd := exec.Command("git", "branch", "-v")
	cmd.Dir = dir
	var b bytes.Buffer
	cmd.Stdout = &b
	if err := cmd.Run(); err != nil {
		return "", err
	}
	lines := strings.Split(b.String(), "\n")
	for _, line := range lines {
		if strings.Contains(line, branch) {
			parts := strings.Split(strings.TrimSpace(line), " ")
			return expandSHA(dir, parts[1])
		}
	}
	return "", fmt.Errorf("branch %q not found", branch)
}

func TestWorker_rebase(t *testing.T) {
	tmp, err := setupTestScenario()
	if err != nil {
		t.Fatal(err.Error())
	}

	cache, err := Prepare(tmp, "master")
	if err != nil {
		t.Fatal(err.Error())
	}

	t.Run("on conflict", func(t *testing.T) {
		branch := "conflict"

		defer cache.Cleanup(StringGitWorktree(branch))
		v, err := cache.Worker(branch)
		if err != nil {
			t.Fatal(err.Error())
		}
		w := v.(*Worker)
		dir, err := w.prepare()
		if err != nil {
			t.Fatal(err.Error())
		}
		if _, err := w.rebase(dir); err == nil {
			t.Fatalf("Expected rebase to error due to conflict, but didn't")
		}
	})

	t.Run("is true if rebase is not necessary", func(t *testing.T) {
		branch := "up-2-date"

		defer cache.Cleanup(StringGitWorktree(branch))
		v, err := cache.Worker(branch)
		if err != nil {
			t.Fatal(err.Error())
		}
		w := v.(*Worker)
		dir, err := w.prepare()
		if err != nil {
			t.Fatal(err.Error())
		}
		if ok, err := w.rebase(dir); err != nil || !ok {
			t.Fatalf("Expected rebase to not be necessary")
		}
	})

	t.Run("is false if rebase was successful and necessary", func(t *testing.T) {
		branch := "needs-rebase"

		defer cache.Cleanup(StringGitWorktree(branch))
		v, err := cache.Worker(branch)
		if err != nil {
			t.Fatal(err.Error())
		}
		w := v.(*Worker)
		dir, err := w.prepare()
		if err != nil {
			t.Fatal(err.Error())
		}
		if ok, err := w.rebase(dir); err != nil || ok {
			t.Fatalf("Expected rebase to not be necessary")
		}
	})
}

func TestWorker_update(t *testing.T) {
	tmp, err := setupTestScenario()
	if err != nil {
		t.Fatal(err.Error())
	}

	cache, err := Prepare(tmp, "master")
	if err != nil {
		t.Fatal(err.Error())
	}

	t.Run("checks out latest version of branch", func(t *testing.T) {
		latest, err := latestSHA(tmp, "needs-rebase")
		if err != nil {
			t.Fatal(err.Error())
		}

		v, err := cache.Worker("needs-rebase")
		if err != nil {
			t.Fatal(err.Error())
		}
		w := v.(*Worker)
		dir, err := w.prepare()
		if err != nil {
			t.Fatal(err.Error())
		}

		w.update(dir)

		cachedSHA, err := getSHA(dir)
		if err != nil {
			t.Fatal(err.Error())
		}

		if cachedSHA != latest {
			t.Fatalf("Expected to get rev %q, but. got %q", latest, cachedSHA)
		}
	})

	t.Run("updates local branch /w remote changes", func(t *testing.T) {
		branch := "up-2-date"

		v, err := cache.Worker(branch)
		if err != nil {
			t.Fatal(err.Error())
		}
		w := v.(*Worker)
		dir, err := w.prepare()
		if err != nil {
			t.Fatal(err.Error())
		}

		cmd := exec.Command("./generate-commit.sh", branch, "example", "file1")
		cmd.Dir = tmp
		var b bytes.Buffer
		cmd.Stdout = &b
		if err := cmd.Run(); err != nil {
			t.Fatal(err.Error())
		}

		latest, err := latestSHA(tmp, branch)
		if err != nil {
			t.Fatal(err.Error())
		}

		w.update(dir)

		cachedSHA, err := getSHA(dir)
		if err != nil {
			t.Fatal(err.Error())
		}

		if cachedSHA != latest {
			t.Fatalf("Expected to get rev %q, but. got %q", latest, cachedSHA)
		}
	})
}
