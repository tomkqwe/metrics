// Package buildinfo formats build metadata for application startup.
package buildinfo

import (
	"fmt"
	"io"
)

// Print writes build metadata to w. Defaults are supplied by the caller.
func Print(w io.Writer, version, date, commit string) {
	_, _ = fmt.Fprintf(w, "Build version: %s\nBuild date: %s\nBuild commit: %s\n",
		version, date, commit)
}
