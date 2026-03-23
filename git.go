package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"strings"

	"github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing"
	"github.com/go-git/go-git/v6/plumbing/transport"
	githttp "github.com/go-git/go-git/v6/plumbing/transport/http"
	gitssh "github.com/go-git/go-git/v6/plumbing/transport/ssh"
)

func isHttpUrl(url string) bool {
	return strings.HasPrefix(url, "http")
}

func isEmpty(dir string) bool {
	entries, err := os.ReadDir(dir)
	return err != nil || len(entries) == 0
}

// cloneOrOpen clones config.GitUrl into config.RepoRoot when the directory is
// absent or empty, and opens the existing repository otherwise.
// The returned bool is true when a fresh clone was performed.
func cloneOrOpen(ctx context.Context, config *Config) (*git.Repository, bool, error) {
	if isEmpty(config.RepoRoot) {
		repo, err := cloneRepo(ctx, config)
		if err != nil {
			return nil, false, err
		}

		return repo, true, nil
	}

	repo, err := git.PlainOpen(config.RepoRoot)
	if err != nil {
		return nil, false, err
	}
	return repo, false, nil
}

func cloneRepo(ctx context.Context, config *Config) (*git.Repository, error) {
	logger := FromContext(ctx)

	if config.GitUrl == "" {
		return nil, errors.New("git url is not set")
	}

	auth, err := getAuthMethod(config.GitUrl, config)
	if err != nil {
		return nil, err
	}

	logger.InfoContext(ctx, "cloning repository", "url", config.GitUrl, "dest", config.RepoRoot)
	repo, err := git.PlainCloneContext(ctx, config.RepoRoot, &git.CloneOptions{
		URL:          config.GitUrl,
		Auth:         auth,
		SingleBranch: true,
	})
	if err != nil {
		return nil, err
	}

	if config.EncryptionEnabled {
		worktree, err := repo.Worktree()
		if err != nil {
			return nil, err
		}

		if err := decryptAllFiles(ctx, worktree.Filesystem, "."); err != nil {
			return nil, err
		}
	}

	return repo, nil
}

func getCurrentRemoteUrl(repo *git.Repository) (string, error) {
	remote, err := repo.Remote("origin")
	if err != nil {
		return "", err
	}

	return remote.Config().URLs[0], nil
}

func getAuthMethod(url string, config *Config) (transport.AuthMethod, error) {
	if isHttpUrl(url) {
		// assume its public repo
		if config.GitToken == "" {
			slog.Warn("git token is not set")
			return nil, nil
		}

		return &githttp.TokenAuth{
			Token: config.GitToken,
		}, nil
	} else {
		if config.GitSshKey == "" {
			return nil, errors.New("git ssh key is not set")
		}
		key, err := gitssh.NewPublicKeys(gitssh.DefaultUsername, []byte(config.GitSshKey), "")
		if err != nil {
			return nil, err
		}
		return key, nil
	}
}

func pullUpdate(ctx context.Context, repo *git.Repository, config *Config) ([]string, error) {
	logger := FromContext(ctx)

	currentRef, err := repo.Head()
	if err != nil {
		return nil, err
	}
	logger.DebugContext(ctx, "current", "ref", currentRef)

	worktree, err := repo.Worktree()
	if err != nil {
		return nil, err
	}

	status, err := worktree.Status()
	if err != nil {
		return nil, err
	}

	if !status.IsClean() {
		return nil, errors.New("worktree is not clean")
	}

	var realRemoteUrl string

	if config.GitUrl != "" {
		realRemoteUrl = config.GitUrl
	} else {
		realRemoteUrl, err = getCurrentRemoteUrl(repo)
		if err != nil {
			return nil, err
		}
	}

	auth, err := getAuthMethod(realRemoteUrl, config)
	if err != nil {
		return nil, err
	}

	pullOpts := &git.PullOptions{
		SingleBranch: true,
		RemoteURL:    realRemoteUrl,
		Auth:         auth,
	}

	logger.InfoContext(ctx, "pulling latest changes", "url", realRemoteUrl)
	err = worktree.PullContext(ctx, pullOpts)
	if err != nil {
		if errors.Is(git.NoErrAlreadyUpToDate, err) {
			return nil, nil
		}
		return nil, err
	}
	logger.DebugContext(ctx, "git pull successful")

	updatedRef, err := repo.Head()
	if err != nil {
		return nil, err
	}
	logger.InfoContext(ctx, "updated", "ref", updatedRef)

	changedFiles, err := diffUpdate(ctx, repo, currentRef.Hash(), updatedRef.Hash())
	if err != nil {
		return nil, err
	}

	changedProjects, err := filterComposeProjects(changedFiles, worktree.Filesystem)
	if err != nil {
		return nil, err
	}

	if config.EncryptionEnabled {
		for _, project := range changedProjects {
			files := changedFiles[project]
			ctx := NewContext(ctx, slog.With("project", project))
			if err := decryptFiles(ctx, worktree.Filesystem, files); err != nil {
				return nil, err
			}
		}
	}

	return changedProjects, nil
}

func diffUpdate(ctx context.Context, repo *git.Repository, oldHash, newHash plumbing.Hash) (map[string][]string, error) {
	oldCommit, err := repo.CommitObject(oldHash)
	if err != nil {
		return nil, err
	}

	oldTree, err := oldCommit.Tree()
	if err != nil {
		return nil, err
	}

	newCommit, err := repo.CommitObject(newHash)
	if err != nil {
		return nil, err
	}

	newTree, err := newCommit.Tree()
	if err != nil {
		return nil, err
	}

	changes, err := oldTree.DiffContext(ctx, newTree)
	if err != nil {
		return nil, err
	}

	changedFiles := make(map[string][]string, 0)

	for _, change := range changes {
		path := change.To.Name
		path, left, found := strings.Cut(path, "/")
		if !found {
			continue
		}

		changedFiles[path] = append(changedFiles[path], left)
	}

	return changedFiles, nil
}
