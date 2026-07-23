package main

import (
	"runtime/debug"
	"strings"
)

const developmentVersion = "dev"

// version is set for release archives with -ldflags "-X main.version=<tag>".
var version = developmentVersion

func currentVersion() string {
	buildInfo, ok := debug.ReadBuildInfo()
	return resolveVersion(version, buildInfo, ok)
}

func resolveVersion(linkerVersion string, buildInfo *debug.BuildInfo, buildInfoOK bool) string {
	linkerVersion = strings.TrimSpace(linkerVersion)
	if linkerVersion != "" && linkerVersion != developmentVersion {
		return linkerVersion
	}

	if buildInfoOK && buildInfo != nil {
		moduleVersion := strings.TrimSpace(buildInfo.Main.Version)
		if moduleVersion != "" && moduleVersion != "(devel)" {
			return moduleVersion
		}
	}

	return developmentVersion
}
