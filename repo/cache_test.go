package repo

import (
	"archive/zip"
	"bytes"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"strings"
	"testing"
)

func getSHA(dir string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = dir
	var b bytes.Buffer
	cmd.Stdout = &b
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return strings.TrimSpace(b.String()), nil
}

func setupTestScenario() (string, error) {
	tmp, err := ioutil.TempDir("", "cache")
	if err != nil {
		return "", err
	}
	if err := extract(tmp, "../scenarios/rebase-conflict.zip"); err != nil {
		return "", err
	}
	return tmp, nil
}

func extract(target, src string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		log.Fatal(err)
	}
	defer r.Close()

	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			os.Mkdir(path.Join(target, f.Name), 0755)
			continue
		}

		t, err := os.OpenFile(path.Join(target, f.Name), os.O_RDWR|os.O_TRUNC|os.O_CREATE, 0755)
		if err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		_, err = io.Copy(t, rc)
		if err != nil {
			return err
		}
		rc.Close()
		t.Close()
	}
	return nil
}

func TestPrepare(t *testing.T) {
	t.Run("clones local repositories", func(t *testing.T) {
		tmp, err := setupTestScenario()
		if err != nil {
			t.Fatal(err.Error())
		}
		cache, err := Prepare(tmp, "master")
		if err != nil {
			t.Fatal(err.Error())
		}
		info, err := os.Stat(path.Join(cache.dir, ".git"))
		if err != nil {
			t.Fatal(err.Error())
		}
		if !info.IsDir() {
			t.Fatal("Expected Prepare to create .git, but didn't")
		}
	})

	t.Run("clones remote repositories", func(t *testing.T) {
		if os.Getenv("CLONE_FROM_GITHUB") == "" {
			t.Skip("Skipping remote clone. Set CLONE_FROM_GITHUB to proceed")
		}

		cache, err := Prepare("https://github.com/nicolai86/github-rebase-bot.git", "master")
		if err != nil {
			t.Fatal(err.Error())
		}

		info, err := os.Stat(path.Join(cache.dir, ".git"))
		if err != nil {
			t.Fatal(err.Error())
		}
		if !info.IsDir() {
			t.Fatal("Expected Prepare to create .git, but didn't")
		}
	})
}

func TestCache_update(t *testing.T) {
	tmp, err := setupTestScenario()
	if err != nil {
		t.Fatal(err.Error())
	}

	t.Run("checks out latest version", func(t *testing.T) {
		cache, err := Prepare(tmp, "master")
		if err != nil {
			t.Fatal(err.Error())
		}

		rev, err := cache.Update()
		if err != nil {
			t.Fatal(err.Error())
		}

		cachedSHA, err := getSHA(cache.dir)
		if err != nil {
			t.Fatal(err.Error())
		}

		if cachedSHA != rev {
			t.Fatalf("Expected rev %q, but got %q", rev, cachedSHA)
		}
	})

	t.Run("updates local copy /w remote changes", func(t *testing.T) {
		cache, err := Prepare(tmp, "master")
		if err != nil {
			t.Fatal(err.Error())
		}
		revBeforeUpdate, err := cache.Update()
		if err != nil {
			t.Fatal(err.Error())
		}

		cmd := exec.Command("./generate-commit.sh", "master", "example", "file1")
		cmd.Dir = tmp
		var b bytes.Buffer
		cmd.Stdout = &b
		if err := cmd.Run(); err != nil {
			t.Fatal(err.Error())
		}

		revAfterUpdate, err := cache.Update()
		if err != nil {
			t.Fatal(err.Error())
		}

		if revBeforeUpdate == revAfterUpdate {
			t.Fatal("Expected update to work, but didn't")
		}
	})
}

func TestCache_Cleanup(t *testing.T) {
	tmp, err := setupTestScenario()
	if err != nil {
		t.Fatal(err.Error())
	}
	cache, err := Prepare(tmp, "master")
	if err != nil {
		t.Fatal(err.Error())
	}

	t.Run("cleans up worktree", func(t *testing.T) {
		_, err := cache.Worker("needs-rebase")
		if err != nil {
			t.Fatal(err.Error())
		}
		cache.Cleanup(StringGitWorktree("needs-rebase"))

		cmd := exec.Command("git", "worktree", "list")
		cmd.Dir = cache.dir
		var b bytes.Buffer
		cmd.Stdout = &b
		if err := cmd.Run(); err != nil {
			t.Fatal(err.Error())
		}

		if strings.Count(b.String(), "\n") != 1 {
			t.Fatalf("Expected worktree to contain 1 branch, but contained more: %s\n", b.String())
		}
	})
}

func TestCache_Worker(t *testing.T) {
	tmp, err := setupTestScenario()
	if err != nil {
		t.Fatal(err.Error())
	}

	t.Run("returns cached worker by branch", func(t *testing.T) {
		cache, err := Prepare(tmp, "master")
		if err != nil {
			t.Fatal(err.Error())
		}
		defer os.RemoveAll(cache.dir)

		w1, err1 := cache.Worker("up-2-date")
		if err1 != nil {
			t.Fatal(err1.Error())
		}
		w2, err2 := cache.Worker("up-2-date")
		if err2 != nil {
			t.Fatal(err2.Error())
		}
		if w1 != w2 {
			t.Fatal("Expected identical workers, but got different ones")
		}
	})

	t.Run("returns new workers by branch", func(t *testing.T) {
		cache, err := Prepare(tmp, "master")
		if err != nil {
			t.Fatal(err.Error())
		}
		defer os.RemoveAll(cache.dir)

		w1, err1 := cache.Worker("up-2-date")
		if err1 != nil {
			t.Fatal(err1.Error())
		}
		w2, err2 := cache.Worker("needs-rebase")
		if err2 != nil {
			t.Fatal(err2.Error())
		}
		if w1 == w2 {
			t.Fatal("Expected different workers, but got identical ones")
		}
	})

	os.RemoveAll(tmp)
}
