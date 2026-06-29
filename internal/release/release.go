// Package release checks GitHub for the latest published release so the CLI can
// tell a user they are running an outdated build. It deliberately has no
// dependencies and treats every network/parse failure as non-fatal: an update
// check is a convenience, never a reason to break a command.
package release

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"golang.org/x/mod/semver"
)

// DefaultAPI is the GitHub "latest release" endpoint for this project.
const DefaultAPI = "https://api.github.com/repos/TheMeinerLP/proxmox-license-proxy/releases/latest"

// Info is the subset of a GitHub release the CLI surfaces.
type Info struct {
	Tag string // e.g. "v1.3.0"
	URL string // browser URL of the release
}

// Checker queries a GitHub-compatible releases endpoint. The fields are
// exported so tests can point it at a stub server with a fast client.
type Checker struct {
	URL    string
	Client *http.Client
}

// NewChecker returns a Checker for the project's GitHub releases with a short
// timeout, so a slow or unreachable network cannot hang the CLI for long.
func NewChecker() *Checker {
	return &Checker{URL: DefaultAPI, Client: &http.Client{Timeout: 5 * time.Second}}
}

// Latest fetches the most recent release. The context bounds the whole call.
func (c *Checker) Latest(ctx context.Context) (Info, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.URL, nil)
	if err != nil {
		return Info{}, err
	}
	// GitHub requires a User-Agent and recommends the versioned Accept header.
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "proxmox-license-proxy")

	resp, err := c.Client.Do(req)
	if err != nil {
		return Info{}, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return Info{}, fmt.Errorf("github API returned %s", resp.Status)
	}
	var body struct {
		TagName string `json:"tag_name"`
		HTMLURL string `json:"html_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return Info{}, err
	}
	if body.TagName == "" {
		return Info{}, fmt.Errorf("github API response had no tag_name")
	}
	return Info{Tag: body.TagName, URL: body.HTMLURL}, nil
}

// IsNewer reports whether latest is a strictly newer release than current,
// using full semver ordering (so 1.10.0 > 1.2.0 and 1.2.0 > 1.2.0-rc1). Both
// may carry a leading "v". A current version that is not valid semver (e.g.
// "dev", "") is treated as older than any release, so an update is suggested;
// an unparseable release tag yields false (cannot tell).
func IsNewer(current, latest string) bool {
	lat := normalize(latest)
	if !semver.IsValid(lat) {
		return false
	}
	cur := normalize(current)
	if !semver.IsValid(cur) {
		return true // dev/unknown local build -> any real release is "newer"
	}
	return semver.Compare(lat, cur) > 0
}

// normalize makes a version string acceptable to the semver package, which
// requires a leading "v". Empty/whitespace stays empty (and so invalid).
func normalize(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	if !strings.HasPrefix(v, "v") {
		v = "v" + v
	}
	return v
}
