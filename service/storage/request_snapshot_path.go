package storage

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/QuantumNous/new-api/common"
)

// Call before the startup request-body cache cleanup, as well as when opening
// the spool, so a conflicting configuration cannot delete older accepted jobs.
func ValidateRequestSnapshotSpoolDirectory() error {
	dir := common.GetEnvOrDefaultString("SNAPSHOT_SPOOL_DIR", filepath.Join("data", "request-snapshots"))
	return validateSnapshotSpoolDirectory(dir)
}

func validateSnapshotSpoolDirectory(dir string) error {
	canonical, err := canonicalSnapshotDirectory(dir)
	if err != nil {
		return err
	}
	for _, cacheDir := range []string{common.GetDiskCacheDir(), filepath.Join(os.TempDir(), "new-api-body-cache")} {
		cacheCanonical, err := canonicalSnapshotDirectory(cacheDir)
		if err != nil {
			return err
		}
		if canonical == cacheCanonical {
			return errors.New("snapshot spool must not share the request-body cache directory")
		}
		spoolInfo, spoolErr := os.Stat(canonical)
		cacheInfo, cacheErr := os.Stat(cacheCanonical)
		if spoolErr == nil && cacheErr == nil && os.SameFile(spoolInfo, cacheInfo) {
			return errors.New("snapshot spool must not share the request-body cache directory")
		}
	}
	return nil
}

// Resolve aliases even when the leaf directory does not exist yet. Reattach
// missing path components only after resolving the closest existing parent.
func canonicalSnapshotDirectory(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	parent := filepath.Clean(absolute)
	var suffix []string
	for {
		resolved, err := filepath.EvalSymlinks(parent)
		if err == nil {
			for i := len(suffix) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, suffix[i])
			}
			return filepath.Clean(resolved), nil
		}
		if !errors.Is(err, os.ErrNotExist) || filepath.Dir(parent) == parent {
			return "", err
		}
		suffix = append(suffix, filepath.Base(parent))
		parent = filepath.Dir(parent)
	}
}
