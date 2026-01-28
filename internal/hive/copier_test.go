package hive

import (
	"bytes"
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/hay-kot/hive/internal/core/config"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileCopier_CopyFiles(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		rules      []config.CopyRule
		remote     string
		setupFiles map[string]string // relative path -> content
		wantFiles  map[string]string // relative path -> content
		wantErr    bool
	}{
		{
			name:       "empty rules",
			rules:      nil,
			remote:     "https://github.com/test/repo",
			setupFiles: map[string]string{"test.txt": "content"},
			wantFiles:  nil,
		},
		{
			name: "empty pattern matches all",
			rules: []config.CopyRule{
				{Pattern: "", Files: []string{"test.txt"}},
			},
			remote:     "https://github.com/test/repo",
			setupFiles: map[string]string{"test.txt": "content"},
			wantFiles:  map[string]string{"test.txt": "content"},
		},
		{
			name: "pattern matches",
			rules: []config.CopyRule{
				{Pattern: ".*/test/repo", Files: []string{"test.txt"}},
			},
			remote:     "https://github.com/test/repo",
			setupFiles: map[string]string{"test.txt": "content"},
			wantFiles:  map[string]string{"test.txt": "content"},
		},
		{
			name: "pattern does not match",
			rules: []config.CopyRule{
				{Pattern: ".*/other/repo", Files: []string{"test.txt"}},
			},
			remote:     "https://github.com/test/repo",
			setupFiles: map[string]string{"test.txt": "content"},
			wantFiles:  nil,
		},
		{
			name: "single file copy",
			rules: []config.CopyRule{
				{Pattern: "", Files: []string{".envrc"}},
			},
			remote:     "https://github.com/test/repo",
			setupFiles: map[string]string{".envrc": "export FOO=bar"},
			wantFiles:  map[string]string{".envrc": "export FOO=bar"},
		},
		{
			name: "wildcard pattern",
			rules: []config.CopyRule{
				{Pattern: "", Files: []string{"*.txt"}},
			},
			remote: "https://github.com/test/repo",
			setupFiles: map[string]string{
				"a.txt":  "a content",
				"b.txt":  "b content",
				"c.json": "json content",
			},
			wantFiles: map[string]string{
				"a.txt": "a content",
				"b.txt": "b content",
			},
		},
		{
			name: "doublestar pattern",
			rules: []config.CopyRule{
				{Pattern: "", Files: []string{"configs/**/*.yaml"}},
			},
			remote: "https://github.com/test/repo",
			setupFiles: map[string]string{
				"configs/dev/app.yaml":  "dev config",
				"configs/prod/app.yaml": "prod config",
				"configs/prod/db.yaml":  "db config",
				"configs/readme.txt":    "readme",
				"other/config.yaml":     "other",
			},
			wantFiles: map[string]string{
				"configs/dev/app.yaml":  "dev config",
				"configs/prod/app.yaml": "prod config",
				"configs/prod/db.yaml":  "db config",
			},
		},
		{
			name: "glob matches nothing warns but continues",
			rules: []config.CopyRule{
				{Pattern: "", Files: []string{"nonexistent.txt", "exists.txt"}},
			},
			remote:     "https://github.com/test/repo",
			setupFiles: map[string]string{"exists.txt": "content"},
			wantFiles:  map[string]string{"exists.txt": "content"},
		},
		{
			name: "multiple rules",
			rules: []config.CopyRule{
				{Pattern: "", Files: []string{".envrc"}},
				{Pattern: ".*/test/repo", Files: []string{".tool-versions"}},
			},
			remote: "https://github.com/test/repo",
			setupFiles: map[string]string{
				".envrc":         "envrc content",
				".tool-versions": "golang 1.21",
			},
			wantFiles: map[string]string{
				".envrc":         "envrc content",
				".tool-versions": "golang 1.21",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Create temp directories
			sourceDir := t.TempDir()
			destDir := t.TempDir()

			// Setup source files
			for path, content := range tt.setupFiles {
				fullPath := filepath.Join(sourceDir, path)
				require.NoError(t, os.MkdirAll(filepath.Dir(fullPath), fs.ModePerm))
				require.NoError(t, os.WriteFile(fullPath, []byte(content), 0o644))
			}

			// Create copier
			var buf bytes.Buffer
			log := zerolog.New(&buf).Level(zerolog.DebugLevel)
			copier := NewFileCopier(log, &buf)

			// Run copy
			err := copier.CopyFiles(context.Background(), tt.rules, tt.remote, sourceDir, destDir)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			// Verify files
			for path, wantContent := range tt.wantFiles {
				fullPath := filepath.Join(destDir, path)
				content, err := os.ReadFile(fullPath)
				require.NoError(t, err, "expected file %s to exist", path)
				assert.Equal(t, wantContent, string(content))
			}

			// Verify no extra files (only check if wantFiles is specified)
			if tt.wantFiles != nil {
				fileCount := 0
				err := filepath.WalkDir(destDir, func(path string, d fs.DirEntry, err error) error {
					if err != nil {
						return err
					}
					if !d.IsDir() {
						fileCount++
					}
					return nil
				})
				require.NoError(t, err)
				assert.Equal(t, len(tt.wantFiles), fileCount, "unexpected number of files in dest")
			}
		})
	}
}

func TestFileCopier_PreservesPermissions(t *testing.T) {
	t.Parallel()

	sourceDir := t.TempDir()
	destDir := t.TempDir()

	// Create executable file
	srcPath := filepath.Join(sourceDir, "script.sh")
	require.NoError(t, os.WriteFile(srcPath, []byte("#!/bin/bash"), 0o755))

	var buf bytes.Buffer
	log := zerolog.New(&buf).Level(zerolog.DebugLevel)
	copier := NewFileCopier(log, &buf)

	rules := []config.CopyRule{
		{Pattern: "", Files: []string{"script.sh"}},
	}

	err := copier.CopyFiles(context.Background(), rules, "remote", sourceDir, destDir)
	require.NoError(t, err)

	// Check permissions
	dstPath := filepath.Join(destDir, "script.sh")
	info, err := os.Stat(dstPath)
	require.NoError(t, err)
	assert.Equal(t, fs.FileMode(0o755), info.Mode().Perm())
}

func TestFileCopier_OverwritesExisting(t *testing.T) {
	t.Parallel()

	sourceDir := t.TempDir()
	destDir := t.TempDir()

	// Create source file
	srcPath := filepath.Join(sourceDir, "config.txt")
	require.NoError(t, os.WriteFile(srcPath, []byte("new content"), 0o644))

	// Create existing dest file
	dstPath := filepath.Join(destDir, "config.txt")
	require.NoError(t, os.WriteFile(dstPath, []byte("old content"), 0o644))

	var buf bytes.Buffer
	log := zerolog.New(&buf).Level(zerolog.DebugLevel)
	copier := NewFileCopier(log, &buf)

	rules := []config.CopyRule{
		{Pattern: "", Files: []string{"config.txt"}},
	}

	err := copier.CopyFiles(context.Background(), rules, "remote", sourceDir, destDir)
	require.NoError(t, err)

	// Verify overwritten
	content, err := os.ReadFile(dstPath)
	require.NoError(t, err)
	assert.Equal(t, "new content", string(content))
}

func TestFileCopier_CreatesParentDirectories(t *testing.T) {
	t.Parallel()

	sourceDir := t.TempDir()
	destDir := t.TempDir()

	// Create nested source file
	nestedPath := filepath.Join(sourceDir, "a/b/c/file.txt")
	require.NoError(t, os.MkdirAll(filepath.Dir(nestedPath), fs.ModePerm))
	require.NoError(t, os.WriteFile(nestedPath, []byte("nested"), 0o644))

	var buf bytes.Buffer
	log := zerolog.New(&buf).Level(zerolog.DebugLevel)
	copier := NewFileCopier(log, &buf)

	rules := []config.CopyRule{
		{Pattern: "", Files: []string{"a/b/c/file.txt"}},
	}

	err := copier.CopyFiles(context.Background(), rules, "remote", sourceDir, destDir)
	require.NoError(t, err)

	// Verify file exists
	dstPath := filepath.Join(destDir, "a/b/c/file.txt")
	content, err := os.ReadFile(dstPath)
	require.NoError(t, err)
	assert.Equal(t, "nested", string(content))
}

func TestFileCopier_RespectsContextCancellation(t *testing.T) {
	t.Parallel()

	sourceDir := t.TempDir()
	destDir := t.TempDir()

	// Create source file
	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, "test.txt"), []byte("content"), 0o644))

	var buf bytes.Buffer
	log := zerolog.New(&buf).Level(zerolog.DebugLevel)
	copier := NewFileCopier(log, &buf)

	rules := []config.CopyRule{
		{Pattern: "", Files: []string{"test.txt"}},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := copier.CopyFiles(ctx, rules, "remote", sourceDir, destDir)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestFileCopier_CopiesSymlink(t *testing.T) {
	t.Parallel()

	sourceDir := t.TempDir()
	destDir := t.TempDir()

	// Create a file and a symlink to it
	targetPath := filepath.Join(sourceDir, "target.txt")
	require.NoError(t, os.WriteFile(targetPath, []byte("target content"), 0o644))

	linkPath := filepath.Join(sourceDir, "link.txt")
	require.NoError(t, os.Symlink("target.txt", linkPath))

	var buf bytes.Buffer
	log := zerolog.New(&buf).Level(zerolog.DebugLevel)
	copier := NewFileCopier(log, &buf)

	rules := []config.CopyRule{
		{Pattern: "", Files: []string{"link.txt"}},
	}

	err := copier.CopyFiles(context.Background(), rules, "remote", sourceDir, destDir)
	require.NoError(t, err)

	// Verify the destination is a symlink with the same target
	dstPath := filepath.Join(destDir, "link.txt")
	info, err := os.Lstat(dstPath)
	require.NoError(t, err)
	assert.NotEqual(t, os.FileMode(0), info.Mode()&os.ModeSymlink, "expected symlink")

	target, err := os.Readlink(dstPath)
	require.NoError(t, err)
	assert.Equal(t, "target.txt", target)
}

func TestFileCopier_CopiesSymlinkWithAbsoluteTarget(t *testing.T) {
	t.Parallel()

	sourceDir := t.TempDir()
	destDir := t.TempDir()

	// Create a symlink with an absolute path target
	absTarget := "/etc/hosts"
	linkPath := filepath.Join(sourceDir, "abs-link")
	require.NoError(t, os.Symlink(absTarget, linkPath))

	var buf bytes.Buffer
	log := zerolog.New(&buf).Level(zerolog.DebugLevel)
	copier := NewFileCopier(log, &buf)

	rules := []config.CopyRule{
		{Pattern: "", Files: []string{"abs-link"}},
	}

	err := copier.CopyFiles(context.Background(), rules, "remote", sourceDir, destDir)
	require.NoError(t, err)

	// Verify the symlink target is preserved
	dstPath := filepath.Join(destDir, "abs-link")
	target, err := os.Readlink(dstPath)
	require.NoError(t, err)
	assert.Equal(t, absTarget, target)
}

func TestFileCopier_OverwritesExistingSymlink(t *testing.T) {
	t.Parallel()

	sourceDir := t.TempDir()
	destDir := t.TempDir()

	// Create source symlink
	srcLink := filepath.Join(sourceDir, "link")
	require.NoError(t, os.Symlink("new-target", srcLink))

	// Create existing destination symlink with different target
	dstLink := filepath.Join(destDir, "link")
	require.NoError(t, os.Symlink("old-target", dstLink))

	var buf bytes.Buffer
	log := zerolog.New(&buf).Level(zerolog.DebugLevel)
	copier := NewFileCopier(log, &buf)

	rules := []config.CopyRule{
		{Pattern: "", Files: []string{"link"}},
	}

	err := copier.CopyFiles(context.Background(), rules, "remote", sourceDir, destDir)
	require.NoError(t, err)

	// Verify the symlink was overwritten
	target, err := os.Readlink(dstLink)
	require.NoError(t, err)
	assert.Equal(t, "new-target", target)
}

func TestIsPathTraversal(t *testing.T) {
	t.Parallel()

	tests := []struct {
		path     string
		expected bool
	}{
		{"normal/path", false},
		{"file.txt", false},
		{"dir/subdir/file.txt", false},
		{".hidden", false},
		{".hidden/file", false},
		{"...", false},
		{"...file", false},
		// Path traversal attempts
		{"../escape", true},
		{"../../escape", true},
		{"dir/../../../escape", true},
		{"/absolute/path", true},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, isPathTraversal(tt.path))
		})
	}
}
