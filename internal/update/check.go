package update

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	// RepoURL is the public project page.
	RepoURL = "https://github.com/Foxemsx/riptide"
	// ReleasesURL is the releases list page.
	ReleasesURL = "https://github.com/Foxemsx/riptide/releases"
	apiLatest   = "https://api.github.com/repos/Foxemsx/riptide/releases/latest"
)

// Result is the outcome of a non-blocking update check.
type Result struct {
	Current         string
	Latest          string
	UpdateAvailable bool
	// OpenURL is where a click should go (release page or repo).
	OpenURL string
	Err     error
}

type ghRelease struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
}

// Check queries GitHub for the latest release and compares it to current.
// Safe to call with a short-lived context; never panics.
func Check(ctx context.Context, current string) Result {
	r := Result{
		Current: current,
		OpenURL: RepoURL,
	}
	if current == "" {
		r.Current = "dev"
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiLatest, nil)
	if err != nil {
		r.Err = err
		return r
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "riptide/"+r.Current)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		r.Err = err
		return r
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		r.Err = fmt.Errorf("github api status %d", resp.StatusCode)
		return r
	}

	var rel ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		r.Err = err
		return r
	}
	if rel.TagName == "" {
		r.Err = fmt.Errorf("empty tag")
		return r
	}

	r.Latest = rel.TagName
	if rel.HTMLURL != "" {
		r.OpenURL = rel.HTMLURL
	} else {
		r.OpenURL = ReleasesURL
	}

	// Only claim an update when both sides parse as versions and latest is newer.
	if canCompare(r.Current) && isNewer(r.Latest, r.Current) {
		r.UpdateAvailable = true
	} else {
		// Up to date (or uncomparable build) — still link to the repo.
		r.OpenURL = RepoURL
	}
	return r
}

func canCompare(v string) bool {
	n := normalize(v)
	return n != "" && n != "dev"
}

func normalize(v string) string {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "v")
	v = strings.TrimPrefix(v, "V")
	// git describe: 1.3.1-3-gabcdef[-dirty] → 1.3.1
	if i := strings.IndexByte(v, '-'); i >= 0 {
		v = v[:i]
	}
	if i := strings.IndexByte(v, '+'); i >= 0 {
		v = v[:i]
	}
	return v
}

// isNewer reports whether latest is strictly greater than current.
func isNewer(latest, current string) bool {
	a := parseSemver(normalize(latest))
	b := parseSemver(normalize(current))
	if a == nil || b == nil {
		return false
	}
	for i := 0; i < 3; i++ {
		if a[i] > b[i] {
			return true
		}
		if a[i] < b[i] {
			return false
		}
	}
	return false
}

func parseSemver(v string) []int {
	if v == "" || v == "dev" {
		return nil
	}
	parts := strings.Split(v, ".")
	out := []int{0, 0, 0}
	for i := 0; i < 3 && i < len(parts); i++ {
		n, err := strconv.Atoi(parts[i])
		if err != nil {
			// strip non-numeric suffix (e.g. "1rc1")
			num := ""
			for _, c := range parts[i] {
				if c >= '0' && c <= '9' {
					num += string(c)
				} else {
					break
				}
			}
			if num == "" {
				return nil
			}
			n, _ = strconv.Atoi(num)
		}
		out[i] = n
	}
	return out
}
