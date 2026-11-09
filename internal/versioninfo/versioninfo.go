// Package versioninfo resolves the build version reported by the CLI.
package versioninfo

import "runtime/debug"

const devVersion = "dev"

// Resolve returns the version to report. An explicitly injected version wins;
// otherwise the module version embedded by the Go toolchain is used. The
// latter is set for binaries installed with "go install module@version".
func Resolve(injected, moduleVersion string) string {
	if injected != "" && injected != devVersion {
		return injected
	}

	if moduleVersion == "" || moduleVersion == "(devel)" {
		return devVersion
	}

	return moduleVersion
}

// ModuleVersion returns the main module version embedded by the Go toolchain.
// It returns an empty string when build information is unavailable.
func ModuleVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}

	return info.Main.Version
}
