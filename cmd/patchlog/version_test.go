package main

import (
	"runtime/debug"
	"testing"
)

func TestResolveVersion(t *testing.T) {
	tests := []struct {
		name          string
		linkerVersion string
		buildInfo     *debug.BuildInfo
		buildInfoOK   bool
		want          string
	}{
		{
			name:          "linker version takes precedence",
			linkerVersion: "v9.8.7",
			buildInfo:     buildInfoWithVersion("v0.1.2"),
			buildInfoOK:   true,
			want:          "v9.8.7",
		},
		{
			name:          "development version falls back to module version",
			linkerVersion: developmentVersion,
			buildInfo:     buildInfoWithVersion("v0.1.2"),
			buildInfoOK:   true,
			want:          "v0.1.2",
		},
		{
			name:          "blank linker version falls back to module version",
			linkerVersion: " ",
			buildInfo:     buildInfoWithVersion(" v0.1.2 "),
			buildInfoOK:   true,
			want:          "v0.1.2",
		},
		{
			name:          "unavailable build info keeps development version",
			linkerVersion: developmentVersion,
			buildInfoOK:   false,
			want:          developmentVersion,
		},
		{
			name:          "nil build info keeps development version",
			linkerVersion: developmentVersion,
			buildInfoOK:   true,
			want:          developmentVersion,
		},
		{
			name:          "development module version is ignored",
			linkerVersion: developmentVersion,
			buildInfo:     buildInfoWithVersion("(devel)"),
			buildInfoOK:   true,
			want:          developmentVersion,
		},
		{
			name:          "blank module version is ignored",
			linkerVersion: developmentVersion,
			buildInfo:     buildInfoWithVersion(" "),
			buildInfoOK:   true,
			want:          developmentVersion,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveVersion(tt.linkerVersion, tt.buildInfo, tt.buildInfoOK); got != tt.want {
				t.Fatalf("resolveVersion() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCurrentVersionUsesDevelopmentVersionForLocalBuild(t *testing.T) {
	if got := currentVersion(); got != developmentVersion {
		t.Fatalf("currentVersion() = %q, want %q for a local test binary", got, developmentVersion)
	}
}

func buildInfoWithVersion(moduleVersion string) *debug.BuildInfo {
	return &debug.BuildInfo{Main: debug.Module{Version: moduleVersion}}
}
