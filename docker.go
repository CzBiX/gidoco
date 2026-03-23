package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"regexp"

	"github.com/docker/cli/cli/command"
	"github.com/docker/cli/cli/flags"
	"github.com/docker/compose/v5/pkg/api"
	"github.com/docker/compose/v5/pkg/compose"
	"github.com/go-git/go-billy/v6"
)

var composeFilePattern = regexp.MustCompile(`^(docker-)?compose(\.enc)?\.ya?ml$`)

func isComposeProject(dir string, fs billy.Filesystem) (bool, error) {
	// TODO: handle project delete

	subFS, err := fs.Chroot(dir)
	if err != nil {
		return false, err
	}

	entries, err := subFS.ReadDir(".")
	if err != nil {
		return false, err
	}

	for _, entry := range entries {
		if composeFilePattern.MatchString(entry.Name()) {
			return true, nil
		}
	}

	return false, nil
}

func filterComposeProjects(changedFiles map[string][]string, fs billy.Filesystem) ([]string, error) {
	dirs := make([]string, 0, len(changedFiles))

	for dir := range changedFiles {
		isCompose, err := isComposeProject(dir, fs)
		if err != nil {
			return nil, fmt.Errorf("failed to check if directory is a compose project: %w", err)
		}
		if isCompose {
			dirs = append(dirs, dir)
		}
	}
	return dirs, nil
}

func allComposeProjects(repoRoot string, fs billy.Filesystem) ([]string, error) {
	entries, err := os.ReadDir(repoRoot)
	if err != nil {
		return nil, err
	}

	var dirs []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		isCompose, err := isComposeProject(entry.Name(), fs)
		if err != nil {
			return nil, fmt.Errorf("failed to check if directory is a compose project: %w", err)
		}
		if isCompose {
			dirs = append(dirs, entry.Name())
		}
	}
	return dirs, nil
}

type logEventProcessor struct {
	logger *slog.Logger
}

func newEventProcessor(logger *slog.Logger) api.EventProcessor {
	return &logEventProcessor{logger: logger}
}

func (p logEventProcessor) Start(ctx context.Context, operation string) {
}

func (p logEventProcessor) On(events ...api.Resource) {
	for _, event := range events {
		p.logger.Debug("docker event", "id", event.ID, "text", event.Text, "details", event.Details)
	}
}

func (p logEventProcessor) Done(operation string, success bool) {
}

func newComposeService(ctx context.Context, config *Config) (api.Compose, error) {
	logger := FromContext(ctx)

	dockerCli, err := command.NewDockerCli(command.WithBaseContext(ctx))
	if err != nil {
		return nil, err
	}

	if err := dockerCli.Initialize(&flags.ClientOptions{}); err != nil {
		return nil, err
	}

	composeOpts := []compose.Option{}
	if config.Debug {
		composeOpts = append(composeOpts, compose.WithEventProcessor(newEventProcessor(logger)))
	}
	if config.DryRun {
		composeOpts = append(composeOpts, compose.WithDryRun)
	}

	service, err := compose.NewComposeService(dockerCli, composeOpts...)
	if err != nil {
		return nil, err
	}

	return service, nil
}
