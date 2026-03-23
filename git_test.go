package main

import (
	"context"
	"maps"
	"slices"
	"testing"
	"time"

	"github.com/go-git/go-billy/v6/memfs"
	"github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing"
	"github.com/go-git/go-git/v6/plumbing/object"
	"github.com/go-git/go-git/v6/storage/memory"
)

// initTestRepo creates an in-memory git repository with a worktree and an
// initial empty commit, returning the repo and the hash of that commit.
func initTestRepo(t *testing.T) (*git.Repository, plumbing.Hash) {
	t.Helper()

	fs := memfs.New()
	store := memory.NewStorage()

	repo, err := git.Init(store, git.WithWorkTree(fs))
	if err != nil {
		t.Fatalf("git.Init: %v", err)
	}

	wt, err := repo.Worktree()
	if err != nil {
		t.Fatalf("Worktree: %v", err)
	}

	// Create a dummy file so the initial commit is not empty.
	f, err := fs.Create(".gitkeep")
	if err != nil {
		t.Fatalf("create .gitkeep: %v", err)
	}
	f.Close()

	if _, err := wt.Add(".gitkeep"); err != nil {
		t.Fatalf("Add .gitkeep: %v", err)
	}

	sig := &object.Signature{
		Name:  "test",
		Email: "test@test.com",
		When:  time.Now(),
	}

	hash, err := wt.Commit("initial", &git.CommitOptions{
		Author: sig,
	})
	if err != nil {
		t.Fatalf("initial commit: %v", err)
	}

	return repo, hash
}

// commitFiles creates files at the given paths in the worktree and commits
// them, returning the new commit hash.
func commitFiles(t *testing.T, repo *git.Repository, paths []string, msg string) plumbing.Hash {
	t.Helper()

	wt, err := repo.Worktree()
	if err != nil {
		t.Fatalf("Worktree: %v", err)
	}

	fs := wt.Filesystem

	for _, p := range paths {
		// Ensure parent directories exist. billy.Filesystem.Create does not
		// auto-create parents, so we derive the directory part manually.
		if dir := parentDir(p); dir != "" {
			if err := fs.MkdirAll(dir, 0o755); err != nil {
				t.Fatalf("MkdirAll %q: %v", dir, err)
			}
		}

		f, err := fs.Create(p)
		if err != nil {
			t.Fatalf("Create %q: %v", p, err)
		}
		// Write a byte so the blob is non-empty.
		f.Write([]byte("x"))
		f.Close()

		if _, err := wt.Add(p); err != nil {
			t.Fatalf("Add %q: %v", p, err)
		}
	}

	sig := &object.Signature{
		Name:  "test",
		Email: "test@test.com",
		When:  time.Now(),
	}

	hash, err := wt.Commit(msg, &git.CommitOptions{
		Author: sig,
	})
	if err != nil {
		t.Fatalf("Commit(%q): %v", msg, err)
	}

	return hash
}

func TestDiffUpdate(t *testing.T) {
	tests := []struct {
		name        string
		oldFiles    []string            // files committed in the "old" commit
		newFiles    []string            // files committed in the "new" commit
		expectFiles map[string][]string // expected dir -> changed file paths
	}{
		{
			name:     "single directory changed",
			oldFiles: []string{"app1/main.go"},
			newFiles: []string{"app1/handler.go"},
			expectFiles: map[string][]string{
				"app1": {"handler.go"},
			},
		},
		{
			name:     "multiple directories changed",
			oldFiles: []string{"svc1/main.go"},
			newFiles: []string{"svc1/util.go", "svc2/main.go"},
			expectFiles: map[string][]string{
				"svc1": {"util.go"},
				"svc2": {"main.go"},
			},
		},
		{
			name:     "nested paths collapse to top-level dir",
			oldFiles: []string{"project/cmd/main.go"},
			newFiles: []string{"project/cmd/server.go", "project/pkg/util.go"},
			expectFiles: map[string][]string{
				"project": {"cmd/server.go", "pkg/util.go"},
			},
		},
		{
			name:     "root-level files are excluded",
			oldFiles: []string{"app/main.go"},
			newFiles: []string{"README.md", "app/handler.go"},
			expectFiles: map[string][]string{
				"app": {"handler.go"},
			},
		},
		{
			name:        "only root-level files changed returns empty",
			oldFiles:    []string{"Makefile"},
			newFiles:    []string{"README.md"},
			expectFiles: map[string][]string{},
		},
		{
			name:        "no changes returns empty",
			oldFiles:    []string{"svc/main.go"},
			newFiles:    []string{},
			expectFiles: map[string][]string{},
		},
		{
			name:     "deeply nested changes collapse correctly",
			oldFiles: []string{"a/b/c/d/e.go"},
			newFiles: []string{"a/b/c/d/f.go", "x/y/z.go"},
			expectFiles: map[string][]string{
				"a": {"b/c/d/f.go"},
				"x": {"y/z.go"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, baseHash := initTestRepo(t)

			// Create the "old" commit with the first set of files.
			oldHash := baseHash
			if len(tt.oldFiles) > 0 {
				oldHash = commitFiles(t, repo, tt.oldFiles, "old commit")
			}

			// Create the "new" commit with the second set of files.
			newHash := oldHash
			if len(tt.newFiles) > 0 {
				newHash = commitFiles(t, repo, tt.newFiles, "new commit")
			}

			changedFiles, err := diffUpdate(context.Background(), repo, oldHash, newHash)
			if err != nil {
				t.Fatalf("diffUpdate: %v", err)
			}

			if changedFiles == nil {
				changedFiles = make(map[string][]string)
			}

			// Verify the returned directories match expectations.
			gotDirs := slices.Sorted(maps.Keys(changedFiles))
			expectDirs := slices.Sorted(maps.Keys(tt.expectFiles))

			if !slices.Equal(gotDirs, expectDirs) {
				t.Fatalf("diffUpdate() dirs = %v, want %v", gotDirs, expectDirs)
			}

			// Verify the files within each directory.
			for dir, expectPaths := range tt.expectFiles {
				gotPaths := make([]string, len(changedFiles[dir]))
				copy(gotPaths, changedFiles[dir])
				slices.Sort(gotPaths)

				wantPaths := make([]string, len(expectPaths))
				copy(wantPaths, expectPaths)
				slices.Sort(wantPaths)

				if !slices.Equal(gotPaths, wantPaths) {
					t.Errorf("diffUpdate()[%q] = %v, want %v", dir, gotPaths, wantPaths)
				}
			}
		})
	}
}
