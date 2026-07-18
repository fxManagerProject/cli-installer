// Package archive handles downloading and safely extracting .zip and
// .tar.gz archives. fxManager release assets are zips; CFX server
// artifacts are shipped as both, depending on OS
package archive

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ulikunitz/xz"
)

var httpClient = &http.Client{Timeout: 5 * time.Minute}

// DownloadToTemp streams a URL to a temp file and returns its path.
// The caller is responsible for removing it
func DownloadToTemp(url string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "fxsetup-installer")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("downloading %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("downloading %s: unexpected status %d", url, resp.StatusCode)
	}

	f, err := os.CreateTemp("", "fxsetup-dl-*")
	if err != nil {
		return "", err
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		os.Remove(f.Name())
		return "", fmt.Errorf("writing download to temp file: %w", err)
	}
	return f.Name(), nil
}

// ExtractAuto picks zip or tar.gz extraction based on the URL/filename
// suffix and extracts into destDir (created if missing)
func ExtractAuto(archivePath, sourceURLOrName, destDir string) error {
	lower := strings.ToLower(sourceURLOrName)
	switch {
	case strings.HasSuffix(lower, ".zip"):
		return ExtractZip(archivePath, destDir)
	case strings.HasSuffix(lower, ".tar.gz"), strings.HasSuffix(lower, ".tgz"):
		return ExtractTarGz(archivePath, destDir)
	case strings.HasSuffix(lower, ".tar.xz"), strings.HasSuffix(lower, ".txz"):
		return ExtractTarXz(archivePath, destDir)
	default:
		return fmt.Errorf("don't know how to extract %q (expected .zip, .tar.gz, .tgz, .tar.xz, or .txz)", sourceURLOrName)
	}
}

// ExtractZip extracts a zip archive into destDir, guarding against
// zip-slip (paths that escape destDir via "..")
func ExtractZip(zipPath, destDir string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("opening zip %s: %w", zipPath, err)
	}
	defer r.Close()

	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return err
	}

	for _, f := range r.File {
		target, err := safeJoin(destDir, f.Name)
		if err != nil {
			return err
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}

		if err := extractZipFile(f, target); err != nil {
			return err
		}
	}
	return nil
}

func extractZipFile(f *zip.File, target string) error {
	rc, err := f.Open()
	if err != nil {
		return fmt.Errorf("opening %s inside zip: %w", f.Name, err)
	}
	defer rc.Close()

	out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, f.Mode())
	if err != nil {
		return fmt.Errorf("creating %s: %w", target, err)
	}
	defer out.Close()

	if _, err := io.Copy(out, rc); err != nil {
		return fmt.Errorf("writing %s: %w", target, err)
	}
	return nil
}

// ExtractTarGz extracts a gzip-compressed tarball into destDir, with
// the same zip-slip protection as ExtractZip
func ExtractTarGz(archivePath, destDir string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("opening archive %s: %w", archivePath, err)
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("reading gzip stream: %w", err)
	}
	defer gz.Close()

	return extractTarStream(gz, destDir)
}

// ExtractTarXz extracts an xz-compressed tarball into destDir. FXServer
// Linux artifacts (fx.tar.xz) ship in this format. Decompression is
// pure Go (github.com/ulikunitz/xz) - no system xz/tar binary required,
// so this works the same on a bare Windows box as it does on Linux.
func ExtractTarXz(archivePath, destDir string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("opening archive %s: %w", archivePath, err)
	}
	defer f.Close()

	xzr, err := xz.NewReader(f)
	if err != nil {
		return fmt.Errorf("reading xz stream: %w", err)
	}

	return extractTarStream(xzr, destDir)
}

// extractTarStream reads a (already decompressed) tar stream from r and
// extracts it into destDir, guarding against zip-slip paths. Shared by
// ExtractTarGz and ExtractTarXz, which differ only in the decompression
// layer wrapping the underlying tar format.
func extractTarStream(r io.Reader, destDir string) error {
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return err
	}

	tr := tar.NewReader(r)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("reading tar entry: %w", err)
		}

		target, err := safeJoin(destDir, hdr.Name)
		if err != nil {
			return err
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode))
			if err != nil {
				return fmt.Errorf("creating %s: %w", target, err)
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return fmt.Errorf("writing %s: %w", target, err)
			}
			out.Close()
		case tar.TypeSymlink:
			continue
		}
	}
}

// safeJoin joins destDir with an archive-internal path, rejecting any
// entry that would resolve outside destDir (zip-slip protection)
func safeJoin(destDir, name string) (string, error) {
	cleaned := filepath.Clean(strings.ReplaceAll(name, "\\", "/"))
	if strings.HasPrefix(cleaned, "..") || filepath.IsAbs(cleaned) {
		return "", fmt.Errorf("archive entry %q escapes destination directory", name)
	}
	target := filepath.Join(destDir, cleaned)
	if !strings.HasPrefix(target, filepath.Clean(destDir)+string(os.PathSeparator)) && target != filepath.Clean(destDir) {
		return "", fmt.Errorf("archive entry %q escapes destination directory", name)
	}
	return target, nil
}
