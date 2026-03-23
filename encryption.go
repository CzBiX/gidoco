package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/getsops/sops/v3/cmd/sops/formats"
	"github.com/getsops/sops/v3/decrypt"
	"github.com/go-git/go-billy/v6"
)

var reEncName = regexp.MustCompile(`\benc\.\w+$`)

func decryptFile(ctx context.Context, fs billy.Filesystem, path string) error {
	logger := FromContext(ctx)

	format := formats.FormatForPath(path)
	file, err := fs.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return err
	}

	data, err = decrypt.DataWithFormat(data, format)
	if err != nil {
		return fmt.Errorf("failed to decrypt %q: %w", path, err)
	}

	destPath := strings.Replace(path, ".enc", "", 1)
	logger.DebugContext(ctx, "decrypting", "source", path, "target", destPath)
	file, err = fs.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.Write(data)
	return err
}

/*
decryptAllFiles decrypts all .enc files in the given directory and its subdirectories.
It is used when a fresh clone is performed.
*/
func decryptAllFiles(ctx context.Context, fs billy.Filesystem, dir string) error {
	logger := FromContext(ctx)

	entries, err := fs.ReadDir(dir)
	if err != nil {
		return err
	}

	decrypted := make([]string, 0)
	for _, entry := range entries {
		var path string
		if dir == "." {
			path = entry.Name()
		} else {
			path = dir + "/" + entry.Name()
		}
		if entry.IsDir() {
			if err := decryptAllFiles(ctx, fs, path); err != nil {
				return err
			}
			continue
		}
		if !reEncName.MatchString(path) {
			continue
		}
		if err := decryptFile(ctx, fs, path); err != nil {
			return err
		}
		decrypted = append(decrypted, path)
	}

	if len(decrypted) > 0 {
		logger.InfoContext(ctx, "decrypted", "path", dir, "files", decrypted)
	}

	return nil
}

func decryptFiles(ctx context.Context, fs billy.Filesystem, files []string) error {
	logger := FromContext(ctx)

	decrypted := make([]string, 0)
	for _, file := range files {
		if !reEncName.MatchString(file) {
			continue
		}

		if err := decryptFile(ctx, fs, file); err != nil {
			return err
		}
		decrypted = append(decrypted, file)
	}

	if len(decrypted) > 0 {
		logger.InfoContext(ctx, "decrypted", "files", decrypted)
	}

	return nil
}
