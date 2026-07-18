// Package jgartifacts resolves the currently-recommended FiveM/RedM
// server artifact build from the community-maintained jgscripts
// artifacts API (https://artifacts.jgscripts.com)
package jgartifacts

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const endpoint = "https://artifacts.jgscripts.com/jsonv2"

// Response mirrors the JGArtifactsResp schema:
//
//	interface JGArtifactsResp {
//	  recommendedArtifact: string;
//	  windowsDownloadLink: string;
//	  linuxDownloadLink: string;
//	  brokenArtifacts: { artifact: string; reason: string; }[];
//	}
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

var httpClient = &http.Client{Timeout: 30 * time.Second}

// Fetch retrieves and parses the current recommendation set
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
	if out.WindowsDownloadLink == "" || out.LinuxDownloadLink == "" {
		return nil, fmt.Errorf("jgscripts response missing download links: %+v", out)
	}
	return &out, nil
}

// DownloadLinkFor returns the artifact download URL for the given target OS ("windows" or "linux")
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

// ResolveDownloadURL is the one-call convenience path most callers want:
// fetch the current recommendations and return the download URL for
// the given target OS ("windows" or "linux"), along with the
// recommended artifact/build label (useful for logging, e.g. "7290")
// Added so in future users can provide the artifact version they want to download
func ResolveDownloadURL(targetOS string) (url string, artifactLabel string, err error) {
	resp, err := Fetch()
	if err != nil {
		return "", "", err
	}
	if broken, reason := resp.IsBroken(); broken {
		return "", "", fmt.Errorf("jgscripts currently flags the recommended artifact %s as broken: %s", resp.RecommendedArtifact, reason)
	}
	link, err := resp.DownloadLinkFor(targetOS)
	if err != nil {
		return "", "", err
	}
	return link, resp.RecommendedArtifact, nil
}

// IsBroken reports whether the recommended artifact is itself flagged
// as broken by jgscripts, along with the reason if so. This can happen
// transiently right after a new CFX artifact drops
func (r *Response) IsBroken() (bool, string) {
	for _, b := range r.BrokenArtifacts {
		if b.Artifact == r.RecommendedArtifact {
			return true, b.Reason
		}
	}
	return false, ""
}
