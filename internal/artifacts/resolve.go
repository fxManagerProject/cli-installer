// Package jgartifacts resolves the currently-recommended and broken FiveM/RedM
// server artifact builds from the community-maintained jgscripts
// artifacts API (https://artifacts.jgscripts.com), allowing installation of
// a recommended or specific artifact build.
package artifacts

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	endpoint      = "https://artifacts.jgscripts.com/jsonv2"
	githubTagsURL = "https://api.github.com/repos/citizenfx/fivem/tags?per_page=100"
	downloadBase  = "https://runtime.fivem.net/artifacts/fivem"
)

type Response struct {
	RecommendedArtifact string           `json:"recommendedArtifact"`
	WindowsDownloadLink string           `json:"windowsDownloadLink"`
	LinuxDownloadLink   string           `json:"linuxDownloadLink"`
	BrokenArtifacts     []BrokenArtifact `json:"brokenArtifacts"`
}

type BrokenArtifact struct {
	Artifact string `json:"artifact"`
	Reason   string `json:"reason"`
}

type GitHubTag struct {
	Name   string `json:"name"`
	Commit struct {
		SHA string `json:"sha"`
	} `json:"commit"`
}

type ResolveResult struct {
	URL           string
	ArtifactLabel string
	IsBroken      bool
	BrokenReason  string
}

var httpClient = &http.Client{Timeout: 30 * time.Second}

// Fetch retrieves and parses the current recommendation set from jgscripts.
func Fetch() (*Response, error) {
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "fxsetup-installer")
	req.Header.Set("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching %s: %w", endpoint, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("jgscripts api returned %d: %s", resp.StatusCode, string(body))
	}

	var out Response
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decoding jgscripts response: %w", err)
	}
	return &out, nil
}

// DownloadLinkFor returns the artifact download URL for the target OS from the response.
func (r *Response) DownloadLinkFor(targetOS string) (string, error) {
	switch targetOS {
	case "windows":
		return r.WindowsDownloadLink, nil
	case "linux":
		return r.LinuxDownloadLink, nil
	default:
		return "", fmt.Errorf("unsupported target os %q (expected windows or linux)", targetOS)
	}
}

// IsArtifactBroken checks if a specific artifact build is flagged as broken.
// Handles exact matches ("7290") and range formats ("7200-7210").
func (r *Response) IsArtifactBroken(artifactID string) (bool, string) {
	targetNum, targetErr := strconv.Atoi(strings.TrimSpace(artifactID))

	for _, b := range r.BrokenArtifacts {
		entry := strings.TrimSpace(b.Artifact)

		// Direct string match
		if entry == strings.TrimSpace(artifactID) {
			return true, b.Reason
		}

		// Range match (e.g. "7200-7205")
		if strings.Contains(entry, "-") {
			parts := strings.Split(entry, "-")
			if len(parts) == 2 && targetErr == nil {
				start, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
				end, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
				if err1 == nil && err2 == nil {
					if targetNum >= start && targetNum <= end {
						return true, b.Reason
					}
				}
			}
		}
	}
	return false, ""
}

// resolveOverrideURL queries FiveM's GitHub repository tags to find the commit SHA
// for a specific artifact build and constructs the download URL.
func resolveOverrideURL(targetOS, artifactID string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, githubTagsURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "fxsetup-installer")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("querying github tags: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("github api returned status %d", resp.StatusCode)
	}

	var tags []GitHubTag
	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		return "", fmt.Errorf("decoding github tags: %w", err)
	}

	targetTag := "v1.0.0." + artifactID
	var sha string
	for _, tag := range tags {
		if tag.Name == targetTag {
			sha = tag.Commit.SHA
			break
		}
	}

	if sha == "" {
		return "", fmt.Errorf("artifact build %s not found in recent GitHub release tags", artifactID)
	}

	switch targetOS {
	case "windows":
		return fmt.Sprintf("%s/build_server_windows/master/%s-%s/server.zip", downloadBase, artifactID, sha), nil
	case "linux":
		return fmt.Sprintf("%s/build_proot_linux/master/%s-%s/fx.tar.xz", downloadBase, artifactID, sha), nil
	default:
		return "", fmt.Errorf("unsupported target os %q", targetOS)
	}
}

// Resolve resolves the download URL and checks broken status for either
// the recommended artifact or an explicitly provided override artifact.
func Resolve(targetOS string, overrideArtifact string) (*ResolveResult, error) {
	resp, err := Fetch()
	if err != nil {
		return nil, err
	}

	var isBroken bool = false
	var reason string = ""

	targetArtifact := resp.RecommendedArtifact
	if overrideArtifact != "" {
		targetArtifact = overrideArtifact
		isBroken, reason = resp.IsArtifactBroken(targetArtifact)
	}

	var downloadURL string
	if overrideArtifact != "" {
		downloadURL, err = resolveOverrideURL(targetOS, overrideArtifact)
		if err != nil {
			return nil, err
		}
	} else {
		downloadURL, err = resp.DownloadLinkFor(targetOS)
		if err != nil {
			return nil, err
		}
	}

	return &ResolveResult{
		URL:           downloadURL,
		ArtifactLabel: targetArtifact,
		IsBroken:      isBroken,
		BrokenReason:  reason,
	}, nil
}
