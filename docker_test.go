package main

import (
	"slices"
	"testing"

	"github.com/go-git/go-billy/v6"
	"github.com/go-git/go-billy/v6/memfs"
)

func TestIsComposeProject(t *testing.T) {
	tests := []struct {
		name   string
		dir    string
		setup  func(t *testing.T, fs billy.Filesystem)
		expect bool
	}{
		{
			name: "docker-compose.yml present",
			dir:  "project",
			setup: func(t *testing.T, fs billy.Filesystem) {
				mkdirAll(t, fs, "project")
				touchFile(t, fs, "project/docker-compose.yml")
			},
			expect: true,
		},
		{
			name: "docker-compose.yaml present",
			dir:  "project",
			setup: func(t *testing.T, fs billy.Filesystem) {
				mkdirAll(t, fs, "project")
				touchFile(t, fs, "project/docker-compose.yaml")
			},
			expect: true,
		},
		{
			name: "compose.yml present",
			dir:  "project",
			setup: func(t *testing.T, fs billy.Filesystem) {
				mkdirAll(t, fs, "project")
				touchFile(t, fs, "project/compose.yml")
			},
			expect: true,
		},
		{
			name: "compose.yaml present",
			dir:  "project",
			setup: func(t *testing.T, fs billy.Filesystem) {
				mkdirAll(t, fs, "project")
				touchFile(t, fs, "project/compose.yaml")
			},
			expect: true,
		},
		{
			name: "no compose file",
			dir:  "project",
			setup: func(t *testing.T, fs billy.Filesystem) {
				mkdirAll(t, fs, "project")
				touchFile(t, fs, "project/README.md")
			},
			expect: false,
		},
		{
			name: "empty directory",
			dir:  "project",
			setup: func(t *testing.T, fs billy.Filesystem) {
				mkdirAll(t, fs, "project")
			},
			expect: false,
		},
		{
			name: "compose file in different directory",
			dir:  "other",
			setup: func(t *testing.T, fs billy.Filesystem) {
				mkdirAll(t, fs, "other")
				mkdirAll(t, fs, "project")
				touchFile(t, fs, "project/docker-compose.yml")
			},
			expect: false,
		},
		{
			name: "multiple compose files present",
			dir:  "project",
			setup: func(t *testing.T, fs billy.Filesystem) {
				mkdirAll(t, fs, "project")
				touchFile(t, fs, "project/docker-compose.yml")
				touchFile(t, fs, "project/compose.yaml")
			},
			expect: true,
		},
		{
			name:   "nonexistent directory returns false",
			dir:    "nonexistent",
			setup:  func(t *testing.T, fs billy.Filesystem) {},
			expect: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := memfs.New()
			tt.setup(t, fs)

			got, err := isComposeProject(tt.dir, fs)
			if err != nil {
				t.Error(err)
			}
			if got != tt.expect {
				t.Errorf("isComposeProject(%q) = %v, want %v", tt.dir, got, tt.expect)
			}
		})
	}
}

func TestFilterComposeProjects(t *testing.T) {
	tests := []struct {
		name         string
		changedFiles map[string][]string
		setup        func(t *testing.T, fs billy.Filesystem)
		expect       []string
	}{
		{
			name: "filters to only compose projects",
			changedFiles: map[string][]string{
				"app1": {"main.go"},
				"app2": {"util.go"},
				"app3": {"handler.go"},
			},
			setup: func(t *testing.T, fs billy.Filesystem) {
				for _, d := range []string{"app1", "app2", "app3"} {
					mkdirAll(t, fs, d)
				}
				touchFile(t, fs, "app1/docker-compose.yml")
				touchFile(t, fs, "app3/compose.yaml")
			},
			expect: []string{"app1", "app3"},
		},
		{
			name: "no matching projects",
			changedFiles: map[string][]string{
				"app1": {"README.md"},
				"app2": {"Makefile"},
			},
			setup: func(t *testing.T, fs billy.Filesystem) {
				for _, d := range []string{"app1", "app2"} {
					mkdirAll(t, fs, d)
				}
				touchFile(t, fs, "app1/README.md")
				touchFile(t, fs, "app2/Makefile")
			},
			expect: []string{},
		},
		{
			name: "all matching projects",
			changedFiles: map[string][]string{
				"svc1": {"main.go"},
				"svc2": {"server.go"},
			},
			setup: func(t *testing.T, fs billy.Filesystem) {
				for _, d := range []string{"svc1", "svc2"} {
					mkdirAll(t, fs, d)
				}
				touchFile(t, fs, "svc1/docker-compose.yml")
				touchFile(t, fs, "svc2/compose.yml")
			},
			expect: []string{"svc1", "svc2"},
		},
		{
			name:         "empty input map",
			changedFiles: map[string][]string{},
			setup:        func(t *testing.T, fs billy.Filesystem) {},
			expect:       []string{},
		},
		{
			name: "single matching project",
			changedFiles: map[string][]string{
				"myapp": {"app.go"},
			},
			setup: func(t *testing.T, fs billy.Filesystem) {
				mkdirAll(t, fs, "myapp")
				touchFile(t, fs, "myapp/compose.yaml")
			},
			expect: []string{"myapp"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := memfs.New()
			tt.setup(t, fs)

			got, err := filterComposeProjects(tt.changedFiles, fs)
			if err != nil {
				t.Error(err)
			}

			if got == nil {
				got = []string{}
			}
			slices.Sort(got)

			expect := make([]string, len(tt.expect))
			copy(expect, tt.expect)
			slices.Sort(expect)

			if !slices.Equal(got, expect) {
				t.Errorf("filterComposeProjects() = %v, want %v", got, expect)
			}
		})
	}
}
