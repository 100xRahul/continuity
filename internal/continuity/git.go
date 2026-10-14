package continuity

import (
	"bytes"
	"crypto/sha256"
	"errors"
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
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return "", fmt.Errorf("create repository directory: %w", err)
	}
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
	var files []string
	if err != nil {
		root = directory
		files, err = sourceFiles(root)
	} else {
		files, err = gitSourceFiles(root)
	}
	if err != nil {
		return "", err
	}
	hash := sha256.New()
	for _, relative := range files {
		path := filepath.Join(root, relative)
		info, err := os.Lstat(path)
		if errors.Is(err, os.ErrNotExist) {
			hash.Write([]byte(relative))
			hash.Write([]byte{0})
			hash.Write([]byte("missing"))
			hash.Write([]byte{0})
			continue
		}
		if err != nil {
			return "", err
		}
		hash.Write([]byte(relative))
		hash.Write([]byte{0})
		if info.Mode()&os.ModeSymlink != 0 {
			target, err := os.Readlink(path)
			if err != nil {
				return "", err
			}
			hash.Write([]byte("symlink:"))
			hash.Write([]byte(target))
			hash.Write([]byte{0})
			continue
		}
		if !info.Mode().IsRegular() {
			hash.Write([]byte("nonregular"))
			hash.Write([]byte{0})
			continue
		}
		contents, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		hash.Write(contents)
		hash.Write([]byte{0})
	}
	return fmt.Sprintf("sha256:%x", hash.Sum(nil)), nil
}

func gitSourceFiles(root string) ([]string, error) {
	output, err := git(root, "ls-files", "-co", "--exclude-standard", "-z")
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	files := []string{}
	for _, item := range strings.Split(output, "\x00") {
		relative := filepath.ToSlash(item)
		if relative == "" || ignoredDirectory(relative) || ignoredFile(relative) || seen[relative] {
			continue
		}
		seen[relative] = true
		files = append(files, relative)
	}
	sort.Strings(files)
	return files, nil
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
		if entry.Type()&os.ModeSymlink == 0 && !entry.Type().IsRegular() {
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

// ignoredDirectory also excludes decision records: they are durable project
// memory read at request time, so a new decision must not mark the source
// index stale.
func ignoredDirectory(relative string) bool {
	relative = filepath.ToSlash(relative)
	if relative == DecisionDir || strings.HasPrefix(relative, DecisionDir+"/") {
		return true
	}
	first := strings.Split(relative, "/")[0]
	switch first {
	case ".git", ".agents", ".codex", ".claude", ".cursor", ".github", ".gemini", "integrations", "node_modules", "vendor", "dist", "build", ".continuity-local":
		return true
	default:
		return false
	}
}

func ignoredFile(relative string) bool {
	base := filepath.Base(relative)
	if filepath.ToSlash(relative) == "continuity" {
		return true
	}
	switch base {
	case ".git", StateFilename, PlanFilename, "AGENTS.md", "CLAUDE.md", "GEMINI.md", "opencode.json", ".gitignore", "README.md":
		return true
	default:
		return strings.HasSuffix(base, ".tmp")
	}
}

// GitCommonDir returns the directory every linked worktree of a repository
// shares, where Continuity keeps local, uncommitted session data.
func GitCommonDir(directory string) (string, error) {
	root, err := GitRoot(directory)
	if err != nil {
		return "", err
	}
	output, err := git(root, "rev-parse", "--git-common-dir")
	if err != nil {
		return "", err
	}
	common := strings.TrimSpace(output)
	if common == "" {
		return "", errors.New("Git common directory is empty")
	}
	if !filepath.IsAbs(common) {
		common = filepath.Join(root, common)
	}
	return filepath.Clean(common), nil
}

func LastStateCommit(directory string) string {
	output, err := git(directory, "log", "-1", "--format=%H", "--", StateFilename)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(output)
}

func ResolveCommit(directory, revision string) (string, error) {
	if strings.TrimSpace(revision) == "" {
		return "", errors.New("commit revision is required")
	}
	output, err := git(directory, "rev-parse", "--verify", revision+"^{commit}")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(output), nil
}

type CommitMessage struct {
	Hash    string
	Date    string
	Subject string
	Body    string
}

// RecentCommitMessages returns up to limit commit messages reachable from
// HEAD, newest first, optionally limited to commits that touched paths. A
// repository without commits has no messages.
func RecentCommitMessages(directory string, limit int, paths ...string) ([]CommitMessage, error) {
	if _, err := git(directory, "rev-parse", "--verify", "--quiet", "HEAD"); err != nil {
		return nil, nil
	}
	args := []string{"log", fmt.Sprintf("--max-count=%d", limit), "--date=short", "--format=%h%x1f%ad%x1f%s%x1f%b%x1e"}
	if len(paths) > 0 {
		args = append(append(args, "--"), paths...)
	}
	output, err := git(directory, args...)
	if err != nil {
		return nil, err
	}
	messages := []CommitMessage{}
	for _, record := range strings.Split(output, "\x1e") {
		fields := strings.Split(strings.TrimLeft(record, "\n"), "\x1f")
		if len(fields) != 4 {
			continue
		}
		messages = append(messages, CommitMessage{Hash: fields[0], Date: fields[1], Subject: fields[2], Body: strings.TrimSpace(fields[3])})
	}
	return messages, nil
}

// CommitFiles records a small, explicitly approved metadata change immediately
// so another harness can rely on it without waiting for an unrelated worker
// commit. Callers must save every requested file before invoking it.
func CommitFiles(directory, message string, files ...string) (string, error) {
	if strings.TrimSpace(message) == "" {
		return "", errors.New("commit message is required")
	}
	if len(files) == 0 {
		return "", errors.New("at least one file is required")
	}
	args := append([]string{"add", "--"}, files...)
	if _, err := git(directory, args...); err != nil {
		return "", err
	}
	if _, err := git(directory, "commit", "-m", message); err != nil {
		return "", err
	}
	return ResolveCommit(directory, "HEAD")
}

// EnsureCommitIdentity verifies that Git can create the approval receipt before
// Continuity mutates a plan. It prevents a half-completed approval caused by a
// missing author identity.
func EnsureCommitIdentity(directory string) error {
	if _, err := git(directory, "var", "GIT_AUTHOR_IDENT"); err != nil {
		return fmt.Errorf("Git commit identity is required: %w", err)
	}
	return nil
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

// ChangedPathsSinceState returns only repository-relative path names changed
// since the last committed durable state record. It is suitable for aging
// source-linked facts; the human-oriented ChangedSinceState keeps its status
// prefixes for reconcile output.
func ChangedPathsSinceState(directory string) ([]string, error) {
	commit := LastStateCommit(directory)
	if commit == "" {
		return nil, nil
	}
	output, err := git(directory, "diff", "--name-only", "-z", commit+"..HEAD")
	if err != nil {
		return nil, err
	}
	paths := []string{}
	seen := map[string]bool{}
	for _, item := range strings.Split(output, "\x00") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		path := filepath.ToSlash(filepath.Clean(item))
		if path == "." || path == ".." || strings.HasPrefix(path, "../") || seen[path] {
			continue
		}
		seen[path] = true
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths, nil
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
