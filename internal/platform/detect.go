// Package platform detects the host operating system (and, where it
// matters, architecture)
//
// Detection can also be overridden via the --os flag (see cmd), for
// cases like preparing a Linux server payload from a Windows machine
// before uploading it to a host.
package platform

import (
	"fmt"
	"runtime"
	"strings"
)

// Target is a supported deployment target for the installer.
type Target string

const (
	Windows Target = "windows"
	Linux   Target = "linux"
)

// Detect returns the Target matching the machine fxsetup is currently
// running on. It errors on anything other than Windows or Linux (e.g.
// macOS isn't a supported FXServer host).
func Detect() (Target, error) {
	return FromGOOS(runtime.GOOS)
}

// FromGOOS maps a Go runtime.GOOS value to a Target. Exported
// separately from Detect so the --os override flag and tests can reuse
// the same validation without needing to fake runtime.GOOS.
func FromGOOS(goos string) (Target, error) {
	switch strings.ToLower(goos) {
	case "windows":
		return Windows, nil
	case "linux":
		return Linux, nil
	case "darwin":
		return "", fmt.Errorf("macOS isn't a supported FXServer host; run this on Windows or Linux, or pass --os=linux to prepare a Linux payload for upload")
	default:
		return "", fmt.Errorf("unsupported OS %q (expected windows or linux)", goos)
	}
}

// ParseOverride validates a user-supplied --os flag value
func ParseOverride(raw string) (Target, error) {
	if raw == "" {
		return Detect()
	}
	return FromGOOS(raw)
}

// FxManagerAssetPattern returns the regexp pattern (consumed by
// ghrelease.Release.FindAsset) that identifies the fxManager webpanel
// binary+assets archive for this target among a release's assets.
// fxManager ships one archive per OS (e.g. containing
// "fxmanager-windows.exe" + "public/"), named to include the OS
func (t Target) FxManagerAssetPattern() string {
	return string(t)
}

// FxManagerResourcePattern returns the regexp pattern that identifies
// the FXServer resource archive
func FxManagerResourcePattern() string {
	return "resource"
}

// String implements fmt.Stringer
func (t Target) String() string {
	return string(t)
}

// Valid reports whether t is a known target
func (t Target) Valid() bool {
	return t == Windows || t == Linux
}
