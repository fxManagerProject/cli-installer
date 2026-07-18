// Package downloader provides the single entry point the rest of the
// installer uses to go from "a URL" to "extracted files on disk":
// download to a temp file, extract it (zip/tar.gz/tar.xz, auto-detected),
// clean up the temp file
package downloader

import (
	"fmt"
	"os"

	"github.com/fxManagerProject/cli-installer/internal/archive"
)

// Progress is an optional callback for reporting install progress back
// to a UI layer (TUI, plain stdout, whatever)
type Progress struct {
	OnStart func(url string)
	OnDone  func(url, destDir string)
}

// DownloadAndExtract downloads the file at url and extracts it into
// destDir. The archive format (zip, tar.gz, tar.xz) is inferred from url's
// suffix; if a server doesn't expose the format in the URL (some
// artifact CDNs redirect through opaque paths), pass filenameHint with
// the real filename/extension instead - e.g. from a Content-Disposition
// header or a known naming convention
func DownloadAndExtract(url, destDir string, filenameHint string, progress *Progress) error {
	if progress != nil && progress.OnStart != nil {
		progress.OnStart(url)
	}

	tmpFile, err := archive.DownloadToTemp(url)
	if err != nil {
		return fmt.Errorf("downloading %s: %w", url, err)
	}
	defer os.Remove(tmpFile)

	nameForDetection := filenameHint
	if nameForDetection == "" {
		nameForDetection = url
	}

	if err := archive.ExtractAuto(tmpFile, nameForDetection, destDir); err != nil {
		return fmt.Errorf("extracting %s into %s: %w", url, destDir, err)
	}

	if progress != nil && progress.OnDone != nil {
		progress.OnDone(url, destDir)
	}
	return nil
}

// DownloadAndExtractToTemp downloads and extracts into a freshly
// created temp directory, returning its path. Useful when the caller
// needs to inspect or selectively copy extracted contents (e.g. pulling
// just the "resource" subfolder out of a larger archive) rather than
// extracting directly into a final destination
func DownloadAndExtractToTemp(url string, filenameHint string, progress *Progress) (string, error) {
	dir, err := os.MkdirTemp("", "fxsetup-extract-*")
	if err != nil {
		return "", err
	}
	if err := DownloadAndExtract(url, dir, filenameHint, progress); err != nil {
		os.RemoveAll(dir)
		return "", err
	}
	return dir, nil
}
