// Package buildinfo formats build metadata for application startup.
package buildinfo

import (
	"fmt"
	"io"
)

// Print writes build metadata to w, replacing missing values with N/A.
func Print(w io.Writer, version, date, commit string) {
	_, _ = fmt.Fprintf(w, "Build version: %s\nBuild date: %s\nBuild commit: %s\n",
		valueOrNA(version), valueOrNA(date), valueOrNA(commit))
}

func valueOrNA(value string) string {
	if value == "" {
		return "N/A"
	}
	return value
}
