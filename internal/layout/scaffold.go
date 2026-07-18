// Package layout builds and populates the on-disk folder structure:
//
//	<root>/               - the fxManager panel binary + its assets
//	<root>/fxServer/      - the extracted CFX server artifact
//	<root>/server-data/    - resources/ + server.cfg, what fxManager points at
//
// This only guarantees the three top-level roots exist and
// that the fxManager resource lands in the right spot inside fxServer/
package layout

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Paths holds every directory the installer needs to know about
type Paths struct {
	Root          string
	FxServerDir   string
	ServerDataDir string
	ResourcesDir  string // server-data/resources
	ServerCfgPath string // server-data/server.cfg
	SystemResDir  string // fxServer/citizen/system_resources (linux layout differs, see SystemResourcesPath)
	TargetOS      string
}

// Scaffold creates the top-level fxServer/, server-data/
// directories under root, plus server-data/resources
func Scaffold(root, targetOS string) (*Paths, error) {
	p := &Paths{
		Root:          root,
		FxServerDir:   filepath.Join(root, "fxServer"),
		ServerDataDir: filepath.Join(root, "server-data"),
		TargetOS:      targetOS,
	}
	p.ResourcesDir = filepath.Join(p.ServerDataDir, "resources")
	p.ServerCfgPath = filepath.Join(p.ServerDataDir, "server.cfg")

	for _, dir := range []string{p.FxServerDir, p.ResourcesDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("creating %s: %w", dir, err)
		}
	}
	return p, nil
}

// SystemResourcesPath returns where CFX's built-in resources live inside
// an extracted server artifact. This differs between Windows and Linux
// builds:
//
//	windows: <fxServer>/citizen/system_resources
//	linux:   <fxServer>/alpine/opt/cfx-server/citizen/system_resources
func (p *Paths) SystemResourcesPath() string {
	if p.TargetOS == "linux" {
		return filepath.Join(p.FxServerDir, "alpine", "opt", "cfx-server", "citizen", "system_resources")
	}
	return filepath.Join(p.FxServerDir, "citizen", "system_resources")
}

// FindSystemResources locates citizen/system_resources under fxServer/
// by walking the tree, used as a fallback when SystemResourcesPath() fails
func (p *Paths) FindSystemResources() (string, error) {
	guess := p.SystemResourcesPath()
	if info, err := os.Stat(guess); err == nil && info.IsDir() {
		return guess, nil
	}

	var found string
	err := filepath.WalkDir(p.FxServerDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || found != "" {
			return err
		}
		if d.IsDir() && filepath.Base(path) == "system_resources" && filepath.Base(filepath.Dir(path)) == "citizen" {
			found = path
			return filepath.SkipAll
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if found == "" {
		return "", fmt.Errorf("could not locate citizen/system_resources under %s - the CFX artifact layout may have changed", p.FxServerDir)
	}
	return found, nil
}

// PlaceFxManagerResource copies the extracted fxManager "resource"
// bundle (the FXServer resource) into fxServer/citizen/system_resources/fxManager
func (p *Paths) PlaceFxManagerResource(extractedResourceDir string) error {
	sysRes, err := p.FindSystemResources()
	if err != nil {
		return err
	}
	dest := filepath.Join(sysRes, "fxManager")
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	return copyTree(extractedResourceDir, dest)
}

// RemoveTxAdminResource removes the txAdmin (monitor) resource from
// citizen/system_resources
func (p *Paths) RemoveTxAdminResource() error {
	sysRes, err := p.FindSystemResources()
	if err != nil {
		return err
	}

	dest := filepath.Join(sysRes, "monitor")
	return os.RemoveAll(dest)
}

// copyTree recursively copies src into dst (dst is created if needed)
func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)

		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}

		info, err := d.Info()
		if err != nil {
			return err
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()

		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
		if err != nil {
			return err
		}
		defer out.Close()

		_, err = io.Copy(out, in)
		return err
	})
}
