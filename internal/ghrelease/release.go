// Package ghrelease resolves the latest GitHub release for a repo and
// picks assets out of it by name pattern
package ghrelease

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"time"
)

// Asset is a single downloadable file attached to a release
type Asset struct {
	Name        string `json:"name"`
	DownloadURL string `json:"browser_download_url"`
	Size        int64  `json:"size"`
}

// Release is the subset of the GitHub API release object we care about
type Release struct {
	TagName string  `json:"tag_name"`
	Name    string  `json:"name"`
	Assets  []Asset `json:"assets"`
}

var httpClient = &http.Client{Timeout: 30 * time.Second}

// Latest fetches the latest non-prerelease, non-draft release for
// owner/repo using the public, unauthenticated GitHub REST API
func Latest(owner, repo string) (*Release, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", owner, repo)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	// GitHub's REST API requires a User-Agent header or it 403s
	req.Header.Set("User-Agent", "fxmanager-installer")
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching latest release for %s/%s: %w", owner, repo, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusForbidden {
		return nil, fmt.Errorf("github api rate-limited this request (403) - try again in a few minutes, or set GITHUB_TOKEN")
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("github api returned %d for %s: %s", resp.StatusCode, url, string(body))
	}

	var rel Release
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, fmt.Errorf("decoding release json: %w", err)
	}
	if len(rel.Assets) == 0 {
		return nil, fmt.Errorf("release %s for %s/%s has no assets", rel.TagName, owner, repo)
	}
	return &rel, nil
}

// FindAsset returns the first asset whose name matches the given regexp
// pattern (case-insensitive)
func (r *Release) FindAsset(namePattern string) (*Asset, error) {
	re, err := regexp.Compile("(?i)" + namePattern)
	if err != nil {
		return nil, fmt.Errorf("invalid asset pattern %q: %w", namePattern, err)
	}
	for i := range r.Assets {
		if re.MatchString(r.Assets[i].Name) {
			return &r.Assets[i], nil
		}
	}
	names := make([]string, len(r.Assets))
	for i, a := range r.Assets {
		names[i] = a.Name
	}
	return nil, fmt.Errorf("no asset in release %s matched pattern %q (available: %v)", r.TagName, namePattern, names)
}

// Download streams an asset to the given writer
func Download(url string, w io.Writer) error {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "fxsetup-installer")

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("downloading %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("downloading %s: unexpected status %d", url, resp.StatusCode)
	}

	_, err = io.Copy(w, resp.Body)
	return err
}
