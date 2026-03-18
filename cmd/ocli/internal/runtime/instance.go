package runtime

import "path/filepath"

// CacheRootForState derives the cache root path from a state directory.
func CacheRootForState(stateDir string) string {
	if stateDir == "" {
		return ""
	}
	return filepath.Join(stateDir, "cache")
}
