package updates

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Yeti47/frozenfortress/frozenfortress/core/ccc"
)

// DefaultGitHubReleasesURL lists the releases of the FrozenFortress repository
const DefaultGitHubReleasesURL = "https://api.github.com/repos/Yeti47/frozenfortress/releases?per_page=20"

// releaseTagPattern matches Docker/server release tags only (e.g. "v1.4.1"),
// excluding prerelease suffixes and other tag families such as "android-v0.2.0-beta1".
var releaseTagPattern = regexp.MustCompile(`^v(\d+)\.(\d+)\.(\d+)$`)

type githubRelease struct {
	TagName     string    `json:"tag_name"`
	HTMLURL     string    `json:"html_url"`
	Draft       bool      `json:"draft"`
	Prerelease  bool      `json:"prerelease"`
	PublishedAt time.Time `json:"published_at"`
}

// GitHubUpdateChecker checks the GitHub releases API for newer versions
type GitHubUpdateChecker struct {
	releasesURL    string
	currentVersion string
	httpClient     *http.Client
	logger         ccc.Logger

	mu     sync.RWMutex
	latest *ReleaseInfo
}

// NewGitHubUpdateChecker creates an update checker comparing releases at releasesURL
// against currentVersion (with or without a leading "v").
func NewGitHubUpdateChecker(releasesURL string, currentVersion string, logger ccc.Logger) *GitHubUpdateChecker {
	if logger == nil {
		logger = ccc.NopLogger
	}

	return &GitHubUpdateChecker{
		releasesURL:    releasesURL,
		currentVersion: currentVersion,
		httpClient:     &http.Client{Timeout: 10 * time.Second},
		logger:         logger,
	}
}

// Check implements UpdateChecker
func (c *GitHubUpdateChecker) Check(ctx context.Context) (*ReleaseInfo, error) {
	current, ok := parseVersion("v" + strings.TrimPrefix(c.currentVersion, "v"))
	if !ok {
		c.logger.Debug("Skipping update check: current version is not a release version", "version", c.currentVersion)
		return nil, nil
	}

	releases, err := c.fetchReleases(ctx)
	if err != nil {
		return nil, err
	}

	var newest *githubRelease
	var newestVersion [3]int
	for i := range releases {
		r := &releases[i]
		if r.Draft || r.Prerelease {
			continue
		}
		v, ok := parseVersion(r.TagName)
		if !ok {
			continue
		}
		if newest == nil || compareVersions(v, newestVersion) > 0 {
			newest = r
			newestVersion = v
		}
	}

	var result *ReleaseInfo
	if newest != nil && compareVersions(newestVersion, current) > 0 {
		result = &ReleaseInfo{
			Version:     strings.TrimPrefix(newest.TagName, "v"),
			URL:         newest.HTMLURL,
			PublishedAt: newest.PublishedAt,
		}
	}

	c.mu.Lock()
	c.latest = result
	c.mu.Unlock()

	return result, nil
}

// Latest implements UpdateChecker
func (c *GitHubUpdateChecker) Latest() *ReleaseInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.latest
}

func (c *GitHubUpdateChecker) fetchReleases(ctx context.Context) ([]githubRelease, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.releasesURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create release request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "frozenfortress/"+c.currentVersion)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch releases: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status fetching releases: %s", resp.Status)
	}

	var releases []githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, fmt.Errorf("failed to decode releases: %w", err)
	}

	return releases, nil
}

// parseVersion parses a "vMAJOR.MINOR.PATCH" tag
func parseVersion(tag string) ([3]int, bool) {
	var v [3]int
	m := releaseTagPattern.FindStringSubmatch(tag)
	if m == nil {
		return v, false
	}
	for i := 0; i < 3; i++ {
		n, err := strconv.Atoi(m[i+1])
		if err != nil {
			return v, false
		}
		v[i] = n
	}
	return v, true
}

// compareVersions returns >0 if a is newer than b, <0 if older, 0 if equal
func compareVersions(a, b [3]int) int {
	for i := 0; i < 3; i++ {
		if a[i] != b[i] {
			return a[i] - b[i]
		}
	}
	return 0
}
