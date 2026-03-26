// Package version holds build-time metadata injected via ldflags.
package version

import (
	"fmt"
	"runtime"
)

// Set via -ldflags at build time by GoReleaser.
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

func String() string {
	return fmt.Sprintf("open-cli %s (%s) built %s %s/%s", Version, Commit, Date, runtime.GOOS, runtime.GOARCH)
}
