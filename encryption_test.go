package main

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/go-git/go-billy/v6/memfs"
)

const PRIV_KEY = "AGE-SECRET-KEY-1ENXC4HWC728QN4DTHFS7H7LMAX0L4RWHR220KHCM2FYLCRDF0PUSD3UX0Z"

func TestReEncName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{
			name:  "simple enc prefix",
			input: "config/enc.yaml",
			want:  true,
		},
		{
			name:  "enc prefix with json extension",
			input: "secrets/enc.json",
			want:  true,
		},
		{
			name:  "enc prefix with env extension",
			input: "app/enc.env",
			want:  true,
		},
		{
			name:  "enc prefix nested path",
			input: "a/b/c/enc.toml",
			want:  true,
		},
		{
			name:  "enc prefix at root",
			input: "enc.yaml",
			want:  true,
		},
		{
			name:  "no enc prefix",
			input: "config/secrets.yaml",
			want:  false,
		},
		{
			name:  "enc in middle of name",
			input: "config/myenc.yaml",
			want:  false,
		},
		{
			name:  "enc without dot",
			input: "config/enc",
			want:  false,
		},
		{
			name:  "enc dot but no extension",
			input: "config/enc.",
			want:  false,
		},
		{
			name:  "partial enc prefix",
			input: "config/enc.yaml.bak",
			want:  false,
		},
		{
			name:  "directory named enc with file",
			input: "enc/secrets.yaml",
			want:  false,
		},
		{
			name:  "enc as suffix with dot separator matches",
			input: "config/secrets.enc.yaml",
			want:  true,
		},
		{
			name:  "word boundary match after slash",
			input: "dir/enc.yml",
			want:  true,
		},
		{
			name:  "word boundary match after hyphen",
			input: "dir/my-enc.yml",
			want:  true,
		},
		{
			name:  "word boundary match after underscore",
			input: "dir/my_enc.yml",
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := reEncName.MatchString(tt.input)
			if got != tt.want {
				t.Errorf("reEncName.MatchString(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func readTestEncContents(t *testing.T) ([]byte, []byte, error) {
	t.Helper()

	encryptedContent, err := os.ReadFile("testdata/test.enc.yml")
	if err != nil {
		return nil, nil, err
	}

	decryptedContent, err := os.ReadFile("testdata/test.yml")
	if err != nil {
		return nil, nil, err
	}

	return encryptedContent, decryptedContent, nil
}

func TestDecryptFiles(t *testing.T) {
	fs := memfs.New()

	encryptedContent, decryptedContent, err := readTestEncContents(t)
	if err != nil {
		t.Fatal(err)
	}

	path := "proj/secrets.enc.yml"
	writeFile(t, fs, path, encryptedContent)

	if err := os.Setenv("SOPS_AGE_KEY", PRIV_KEY); err != nil {
		t.Fatal(err)
	}

	if err := decryptFiles(context.Background(), fs, []string{path}); err != nil {
		t.Fatal(err)
	}

	data := readFile(t, fs, "proj/secrets.yml")

	data = bytes.TrimSpace(data)
	decryptedContent = bytes.TrimSpace(decryptedContent)

	if !bytes.Equal(data, decryptedContent) {
		t.Errorf("decrypted file content = %q, want %q", string(data), string(decryptedContent))
	}
}

func TestDecryptFile_FileExists(t *testing.T) {
	fs := memfs.New()

	encryptedContent, decryptedContent, err := readTestEncContents(t)
	if err != nil {
		t.Fatal(err)
	}

	encPath := "proj/secrets.enc.yml"
	writeFile(t, fs, encPath, encryptedContent)

	decPath := "proj/secrets.yml"
	writeFile(t, fs, decPath, []byte("dummy content"))

	if err := os.Setenv("SOPS_AGE_KEY", PRIV_KEY); err != nil {
		t.Fatal(err)
	}

	if err := decryptFile(context.Background(), fs, encPath); err != nil {
		t.Fatal(err)
	}

	data := readFile(t, fs, decPath)

	data = bytes.TrimSpace(data)
	decryptedContent = bytes.TrimSpace(decryptedContent)

	if !bytes.Equal(data, decryptedContent) {
		t.Errorf("decrypted file content = %q, want %q", string(data), string(decryptedContent))
	}
}
