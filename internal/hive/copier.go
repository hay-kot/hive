package hive

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/hay-kot/hive/internal/core/config"
	"github.com/hay-kot/hive/internal/styles"
	"github.com/rs/zerolog"
)

// FileCopier copies files from a source directory to a destination.
type FileCopier struct {
	log    zerolog.Logger
	stdout io.Writer
}

// NewFileCopier creates a new FileCopier.
func NewFileCopier(log zerolog.Logger, stdout io.Writer) *FileCopier {
	return &FileCopier{
		log:    log,
		stdout: stdout,
	}
}

// CopyFiles copies files matching the rules from sourceDir to destDir.
func (c *FileCopier) CopyFiles(ctx context.Context, rules []config.CopyRule, remote, sourceDir, destDir string) error {
	c.log.Debug().
		Str("remote", remote).
		Str("source", sourceDir).
		Str("dest", destDir).
		Int("rule_count", len(rules)).
		Msg("evaluating copy rules")

	ruleNum := 0
	for _, rule := range rules {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		matched, err := c.matchPattern(rule.Pattern, remote)
		if err != nil {
			return fmt.Errorf("match pattern %q: %w", rule.Pattern, err)
		}

		c.log.Debug().
			Str("pattern", rule.Pattern).
			Str("remote", remote).
			Bool("matched", matched).
			Msg("copy rule pattern evaluated")

		if !matched {
			continue
		}

		ruleNum++

		c.log.Debug().
			Str("pattern", rule.Pattern).
			Strs("files", rule.Files).
			Msg("processing copy rule")

		for _, filePattern := range rule.Files {
			if err := c.copyPattern(ctx, ruleNum, sourceDir, destDir, filePattern); err != nil {
				return err
			}
		}
	}

	return nil
}

// matchPattern checks if remote matches the regex pattern.
// Empty pattern matches all remotes.
func (c *FileCopier) matchPattern(pattern, remote string) (bool, error) {
	if pattern == "" {
		return true, nil
	}
	return regexp.MatchString(pattern, remote)
}

// globFiles finds files matching a pattern in sourceDir, including symlinks.
// Returns paths relative to sourceDir.
func (c *FileCopier) globFiles(sourceDir, pattern string) ([]string, error) {
	fullPattern := filepath.Join(sourceDir, pattern)

	// Check if pattern contains glob special characters
	if !hasGlobChars(pattern) {
		// Literal path - check directly with Lstat to include symlinks
		if _, err := os.Lstat(fullPattern); err != nil {
			if os.IsNotExist(err) {
				return nil, nil
			}
			return nil, err
		}
		return []string{pattern}, nil
	}

	// Use FilepathGlob with WithNoFollow to include symlinks
	allMatches, err := doublestar.FilepathGlob(fullPattern, doublestar.WithNoFollow())
	if err != nil {
		return nil, err
	}

	// Convert absolute paths back to relative
	var matches []string
	for _, match := range allMatches {
		rel, err := filepath.Rel(sourceDir, match)
		if err != nil {
			return nil, fmt.Errorf("relative path for %q: %w", match, err)
		}
		matches = append(matches, rel)
	}

	return matches, nil
}

// hasGlobChars returns true if pattern contains glob special characters.
func hasGlobChars(pattern string) bool {
	return strings.ContainsAny(pattern, "*?[{")
}

// copyPattern copies files matching a glob pattern from source to dest.
func (c *FileCopier) copyPattern(ctx context.Context, ruleNum int, sourceDir, destDir, pattern string) error {
	matches, err := c.globFiles(sourceDir, pattern)
	if err != nil {
		return fmt.Errorf("glob %q: %w", pattern, err)
	}

	if len(matches) == 0 {
		c.log.Warn().
			Str("pattern", pattern).
			Str("source", sourceDir).
			Msg("glob pattern matched no files")
		return nil
	}

	c.printCopyHeader(ruleNum, pattern, len(matches))

	for _, match := range matches {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		srcPath := filepath.Join(sourceDir, match)
		dstPath := filepath.Join(destDir, match)

		if err := c.copyFile(srcPath, dstPath); err != nil {
			return fmt.Errorf("copy %q: %w", match, err)
		}

		c.log.Debug().
			Str("src", srcPath).
			Str("dst", dstPath).
			Msg("copied file")

		_, _ = fmt.Fprintf(c.stdout, "  %s\n", match)
	}

	return nil
}

// copyFile copies a single file or symlink, preserving permissions and creating parent directories.
func (c *FileCopier) copyFile(src, dst string) error {
	srcInfo, err := os.Lstat(src)
	if err != nil {
		return fmt.Errorf("lstat source: %w", err)
	}

	// Skip directories - doublestar.Glob can return directory entries
	if srcInfo.IsDir() {
		return nil
	}

	// Create parent directories
	if err := os.MkdirAll(filepath.Dir(dst), fs.ModePerm); err != nil {
		return fmt.Errorf("create parent dirs: %w", err)
	}

	// Handle symlinks
	if srcInfo.Mode()&os.ModeSymlink != 0 {
		return c.copySymlink(src, dst)
	}

	return c.copyRegularFile(src, dst, srcInfo)
}

// copySymlink recreates a symlink at the destination.
func (c *FileCopier) copySymlink(src, dst string) error {
	target, err := os.Readlink(src)
	if err != nil {
		return fmt.Errorf("read symlink: %w", err)
	}

	// Check if destination exists and remove it
	if _, err := os.Lstat(dst); err == nil {
		c.log.Warn().
			Str("path", dst).
			Msg("overwriting existing file")
		if err := os.Remove(dst); err != nil {
			return fmt.Errorf("remove existing: %w", err)
		}
	}

	if err := os.Symlink(target, dst); err != nil {
		return fmt.Errorf("create symlink: %w", err)
	}

	c.log.Debug().
		Str("src", src).
		Str("dst", dst).
		Str("target", target).
		Msg("copied symlink")

	return nil
}

// copyRegularFile copies a regular file preserving permissions.
func (c *FileCopier) copyRegularFile(src, dst string, srcInfo fs.FileInfo) error {
	// Check if destination exists
	if _, err := os.Lstat(dst); err == nil {
		c.log.Warn().
			Str("path", dst).
			Msg("overwriting existing file")
	}

	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open source: %w", err)
	}
	defer srcFile.Close()

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return fmt.Errorf("create destination: %w", err)
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("copy contents: %w", err)
	}

	return nil
}

// printCopyHeader prints a styled header for a copy operation.
func (c *FileCopier) printCopyHeader(ruleNum int, pattern string, count int) {
	divider := styles.DividerStyle.Render(strings.Repeat("─", 50))
	header := styles.CommandHeaderStyle.Render(fmt.Sprintf("copy %d", ruleNum))
	patternLabel := styles.CommandStyle.Render(pattern)
	countLabel := styles.DividerStyle.Render(fmt.Sprintf("[%d files]", count))

	_, _ = fmt.Fprintln(c.stdout)
	_, _ = fmt.Fprintln(c.stdout, divider)
	_, _ = fmt.Fprintf(c.stdout, "%s %s %s\n", header, patternLabel, countLabel)
	_, _ = fmt.Fprintln(c.stdout, divider)
}
