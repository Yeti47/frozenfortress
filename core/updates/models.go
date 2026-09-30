package updates

import "time"

// ReleaseInfo describes a published release of the application
type ReleaseInfo struct {
	Version     string    // Version without the leading "v", e.g. "1.4.1"
	URL         string    // Link to the release page
	PublishedAt time.Time // When the release was published
}
