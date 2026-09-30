package updates

import "context"

// UpdateChecker looks up whether a newer release of the application is available
type UpdateChecker interface {
	// Check queries the release source and caches the result.
	// Returns the newer release, or nil if the running version is up to date.
	Check(ctx context.Context) (*ReleaseInfo, error)

	// Latest returns the newer release found by the last successful Check, or nil.
	// It never blocks on network I/O.
	Latest() *ReleaseInfo
}
