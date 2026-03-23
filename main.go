package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"sync"

	"github.com/compose-spec/compose-go/v2/types"
	"github.com/docker/compose/v5/pkg/api"
	"github.com/gin-gonic/gin"
	"github.com/go-git/go-git/v6"
)

func main() {
	if len(os.Args) > 1 {
		fmt.Fprintf(os.Stderr, "gidoco - Git-driven Docker Compose orchestrator\n\n")
		fmt.Fprintf(os.Stderr, "gidoco does not accept any command-line arguments yet.\n")
		fmt.Fprintf(os.Stderr, "All configuration is done via config.yml or environment variables.\n")
		os.Exit(1)
	}

	config, err := loadConfig()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	slog.Info("working on", "repo", config.RepoRoot)

	ctx := context.Background()
	if err := initialize(ctx, config); err != nil {
		slog.Error("initialization failed", "error", err)
		os.Exit(1)
	}

	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	webhookHandler := func(c *gin.Context) {
		go func() {
			repo, err := git.PlainOpen(config.RepoRoot)
			if err != nil {
				slog.Error("error opening repo", "error", err)
				return
			}
			if err := runUpdate(repo, config); err != nil {
				slog.Error("error running update", "error", err)
			} else {
				slog.Info("update completed successfully")
			}
		}()

		c.JSON(http.StatusOK, gin.H{})
	}

	if config.WebhookId == "" {
		r.POST("/webhook", webhookHandler)
	} else {
		r.POST("/webhook/:hookId", webhookHandler)
	}

	addr := fmt.Sprintf(":%d", config.Port)
	slog.Info("listening", "addr", "http://"+addr)
	if err := r.Run(addr); err != nil {
		slog.Error("failed to start server", "error", err)
		os.Exit(1)
	}
}

func initialize(ctx context.Context, config *Config) error {
	logger := FromContext(ctx)

	repo, freshClone, err := cloneOrOpen(ctx, config)
	if err != nil {
		return err
	}

	if !freshClone && config.NoStartUp {
		return nil
	}

	if !freshClone {
		logger.InfoContext(ctx, "running startup update")
		return runUpdate(repo, config)
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return err
	}

	dirs, err := allComposeProjects(config.RepoRoot, worktree.Filesystem)
	if err != nil {
		return err
	}

	logger.InfoContext(ctx, "starting all compose projects", "count", len(dirs))
	return batchUpProjects(ctx, dirs, config)
}

func batchUpProjects(ctx context.Context, dirs []string, config *Config) error {
	logger := FromContext(ctx)

	repoConfig, err := loadRepoConfig(config.RepoRoot)
	if err != nil {
		return err
	}

	compose, err := newComposeService(ctx, config)
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	errCh := make(chan error, len(dirs))
	sem := make(chan struct{}, 3)

	for _, dir := range dirs {
		if !filterProjects(repoConfig, dir) {
			logger.InfoContext(ctx, "skipping", "project", dir)
			continue
		}

		sem <- struct{}{}

		wg.Go(func() {
			defer func() {
				<-sem
			}()

			fullDir := filepath.Join(config.RepoRoot, dir)
			ctx := NewContext(ctx, slog.With("project", dir))

			if err := composeUp(ctx, compose, fullDir); err != nil {
				errCh <- err
			}
		})
	}
	wg.Wait()
	close(errCh)

	var errs []error
	for err := range errCh {
		errs = append(errs, err)
	}

	return errors.Join(errs...)
}

func composeUp(ctx context.Context, compose api.Compose, dir string) error {
	logger := FromContext(ctx)

	project, err := compose.LoadProject(ctx, api.ProjectLoadOptions{
		WorkingDir: dir,
	})
	if err != nil {
		return err
	}

	// Force pull policy to build for services with build context
	for name, service := range project.Services {
		if service.Build != nil {
			service.PullPolicy = types.PullPolicyBuild
			project.Services[name] = service
		}
	}

	logger.InfoContext(ctx, "starting")
	return compose.Up(ctx, project, api.UpOptions{
		Create: api.CreateOptions{
			RemoveOrphans: true,
		},
	})
}

var updateMu sync.Mutex

func runUpdate(repo *git.Repository, config *Config) error {
	updateMu.Lock()
	defer updateMu.Unlock()

	slog.Info("starting update process", "repoPath", config.RepoRoot)
	ctx := context.Background()

	dirs, err := pullUpdate(ctx, repo, config)
	if err != nil {
		return err
	}

	if len(dirs) == 0 {
		slog.Info("no update needed")
		return nil
	}

	return batchUpProjects(ctx, dirs, config)
}

// true if project should be included
func filterProjects(repoConfig *RepoConfig, project string) bool {
	if len(repoConfig.IncludedProjects) > 0 {
		return slices.Contains(repoConfig.IncludedProjects, project)
	}

	return !slices.Contains(repoConfig.ExcludedProjects, project)
}
