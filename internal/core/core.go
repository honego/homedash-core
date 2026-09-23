package core

import (
	"fmt"
	"runtime"
)

var (
	Version   = "unknown"
	GitCommit = "unknown"
	BuildTime = "unknown"
)

func VersionStatement() string {
	return fmt.Sprintf("homedash %s (%s) (%s %s/%s)", Version, GitCommit, runtime.Version(), runtime.GOOS, runtime.GOARCH)
}
