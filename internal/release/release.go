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
	"strconv"
	"strings"
	"time"
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

// IsNewer reports whether latest is a strictly newer release than current.
// Both may carry a leading "v" and a pre-release/build suffix (ignored). A
// current version that is not a real release (e.g. "dev", "") is treated as
// older than any release, so an update is suggested.
func IsNewer(current, latest string) bool {
	lat, okLatest := parseSemver(latest)
	if !okLatest {
		return false // cannot compare against an unparseable release tag
	}
	cur, okCurrent := parseSemver(current)
	if !okCurrent {
		return true // dev/unknown local build -> any real release is "newer"
	}
	return compare(lat, cur) > 0
}

// parseSemver extracts major.minor.patch from a version string, ignoring a
// leading "v" and any "-pre"/"+build" suffix. Missing components default to 0.
func parseSemver(v string) ([3]int, bool) {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		v = v[:i]
	}
	if v == "" {
		return [3]int{}, false
	}
	parts := strings.Split(v, ".")
	if len(parts) > 3 {
		return [3]int{}, false
	}
	var out [3]int
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return [3]int{}, false
		}
		out[i] = n
	}
	return out, true
}

func compare(a, b [3]int) int {
	for i := range a {
		switch {
		case a[i] > b[i]:
			return 1
		case a[i] < b[i]:
			return -1
		}
	}
	return 0
}
