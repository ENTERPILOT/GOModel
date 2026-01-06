package version

import (
	"fmt"
	"runtime"
)

// These variables are set via -ldflags during the build process
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// Info returns a formatted version string
func Info() string {
	return fmt.Sprintf("gomodel %s (commit: %s, built: %s, %s)", Version, Commit, Date, runtime.Version())
}
