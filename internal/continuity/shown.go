package continuity

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// A hook shows each path-linked memory record once per session. The keys of
// shown records live in Git's common directory with one file per session, so
// concurrent sessions never write the same file.
const shownRecordsDirectory = "sessions"

func shownRecordsPath(directory, owner string) (string, error) {
	if strings.TrimSpace(owner) == "" {
		return "", errors.New("a session identity is required to track shown records")
	}
	store, err := CoordinationRoot(directory)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(owner))
	return filepath.Join(store, shownRecordsDirectory, hex.EncodeToString(sum[:8])), nil
}

func LoadShownRecords(directory, owner string) (map[string]bool, error) {
	path, err := shownRecordsPath(directory, owner)
	if err != nil {
		return nil, err
	}
	shown := map[string]bool{}
	contents, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return shown, nil
	}
	if err != nil {
		return nil, err
	}
	for _, key := range strings.Fields(string(contents)) {
		shown[key] = true
	}
	return shown, nil
}

// AddShownRecords appends keys in one write. Two hooks that run in parallel
// for the same session can at worst show a record twice.
func AddShownRecords(directory, owner string, keys []string) error {
	if len(keys) == 0 {
		return nil
	}
	path, err := shownRecordsPath(directory, owner)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := file.WriteString(strings.Join(keys, "\n") + "\n"); err != nil {
		file.Close()
		return err
	}
	return file.Close()
}

// ResetShownRecords forgets what a session was shown. Compaction and a cleared
// conversation drop earlier context, so linked records must be shown again.
func ResetShownRecords(directory, owner string) error {
	path, err := shownRecordsPath(directory, owner)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// PruneShownRecords removes the files of sessions inactive for longer than age.
func PruneShownRecords(directory string, age time.Duration) error {
	store, err := CoordinationRoot(directory)
	if err != nil {
		return err
	}
	sessions := filepath.Join(store, shownRecordsDirectory)
	entries, err := os.ReadDir(sessions)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	cutoff := time.Now().Add(-age)
	for _, entry := range entries {
		info, err := entry.Info()
		if errors.Is(err, os.ErrNotExist) {
			continue // another session pruned it first
		}
		if err != nil {
			return err
		}
		if info.ModTime().Before(cutoff) {
			if err := os.Remove(filepath.Join(sessions, entry.Name())); err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
		}
	}
	return nil
}
