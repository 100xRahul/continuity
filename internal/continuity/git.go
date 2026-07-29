package continuity

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

type GitSnapshot struct {
	Root         string
	Available    bool
	Branch       string
	Head         string
	Dirty        bool
	ChangedFiles []string
}

func EnsureGitRepo(directory string) (string, error) {
	if _, err := git(directory, "rev-parse", "--show-toplevel"); err == nil {
		return GitRoot(directory)
	}
	if _, err := git(directory, "init", "-b", "main"); err != nil {
		return "", fmt.Errorf("initialize git repository: %w", err)
	}
	return GitRoot(directory)
}

func GitRoot(directory string) (string, error) {
	output, err := git(directory, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(output), nil
}

func Snapshot(directory string) GitSnapshot {
	root, err := GitRoot(directory)
	if err != nil {
		return GitSnapshot{Root: directory}
	}
	snapshot := GitSnapshot{Root: root, Available: true}
	if output, err := git(root, "symbolic-ref", "--short", "HEAD"); err == nil {
		snapshot.Branch = strings.TrimSpace(output)
	}
	if output, err := git(root, "rev-parse", "HEAD"); err == nil {
		snapshot.Head = strings.TrimSpace(output)
	}
	if output, err := git(root, "status", "--porcelain=v1"); err == nil {
		for _, line := range strings.Split(strings.TrimSuffix(output, "\n"), "\n") {
			if len(line) < 4 {
				continue
			}
			snapshot.Dirty = true
			name := strings.TrimSpace(line[3:])
			if before, after, renamed := strings.Cut(name, " -> "); renamed {
				name = after
				_ = before
			}
			snapshot.ChangedFiles = append(snapshot.ChangedFiles, name)
		}
	}
	return snapshot
}

func Fingerprint(directory string) (string, error) {
	root, err := GitRoot(directory)
	if err != nil {
		root = directory
	}
	files, err := sourceFiles(root)
	if err != nil {
		return "", err
	}
	hash := sha256.New()
	for _, relative := range files {
		contents, err := os.ReadFile(filepath.Join(root, relative))
		if err != nil {
			return "", err
		}
		hash.Write([]byte(relative))
		hash.Write([]byte{0})
		hash.Write(contents)
		hash.Write([]byte{0})
	}
	return fmt.Sprintf("sha256:%x", hash.Sum(nil)), nil
}

func sourceFiles(root string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if relative == "." {
			return nil
		}
		if entry.IsDir() {
			if ignoredDirectory(relative) {
				return filepath.SkipDir
			}
			return nil
		}
		if ignoredFile(relative) {
			return nil
		}
		files = append(files, filepath.ToSlash(relative))
		return nil
	})
	sort.Strings(files)
	return files, err
}

func ignoredDirectory(relative string) bool {
	first := strings.Split(filepath.ToSlash(relative), "/")[0]
	switch first {
	case ".git", ".agents", ".codex", ".claude", ".cursor", ".github", ".gemini", "integrations", "node_modules", "vendor", "dist", "build", ".continuity-local":
		return true
	default:
		return false
	}
}

func ignoredFile(relative string) bool {
	base := filepath.Base(relative)
	switch base {
	case StateFilename, "AGENTS.md", "CLAUDE.md", "GEMINI.md", "opencode.json", ".gitignore", "README.md":
		return true
	default:
		return strings.HasSuffix(base, ".tmp")
	}
}

func LastStateCommit(directory string) string {
	output, err := git(directory, "log", "-1", "--format=%H", "--", StateFilename)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(output)
}

func ChangedSinceState(directory string) ([]string, error) {
	commit := LastStateCommit(directory)
	if commit == "" {
		return nil, nil
	}
	output, err := git(directory, "diff", "--name-status", commit+"..HEAD")
	if err != nil {
		return nil, err
	}
	lines := strings.FieldsFunc(strings.TrimSpace(output), func(r rune) bool { return r == '\n' })
	return lines, nil
}

// ReviewAnchors reports durable evidence anchors whose file or directory was
// changed after the last state synchronization or remains dirty now.
func ReviewAnchors(directory string, state *State) []string {
	changed := map[string]bool{}
	for _, path := range Snapshot(directory).ChangedFiles {
		changed[filepath.ToSlash(path)] = true
	}
	if since, err := ChangedSinceState(directory); err == nil {
		for _, line := range since {
			fields := strings.Fields(line)
			if len(fields) > 1 {
				changed[filepath.ToSlash(fields[len(fields)-1])] = true
			}
		}
	}
	review := []string{}
	for _, anchor := range state.EvidencePaths() {
		for path := range changed {
			if path == anchor || strings.HasPrefix(path, strings.TrimSuffix(anchor, "/")+"/") || strings.HasPrefix(anchor, strings.TrimSuffix(path, "/")+"/") {
				review = append(review, anchor)
				break
			}
		}
	}
	sort.Strings(review)
	return review
}

func git(directory string, args ...string) (string, error) {
	command := exec.Command("git", args...)
	command.Dir = directory
	var stderr bytes.Buffer
	command.Stderr = &stderr
	output, err := command.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), strings.TrimSpace(stderr.String()))
	}
	return string(output), nil
}
