// Package domain defines the core business entities and interfaces for slippy-find.
package domain

// GitContext contains all derived git information needed for slip resolution.
// This struct is populated by LocalGitRepository.GetGitContext() from the local repository.
type GitContext struct {
	// HeadSHA is the full 40-character commit SHA of HEAD.
	HeadSHA string

	// Branch is the current branch name (empty string if HEAD is detached).
	Branch string

	// Repository is the repository name in owner/repo format.
	// Derived from the 'origin' remote URL.
	Repository string

	// IsDetached indicates if HEAD is detached (not on a branch).
	// When true, Branch will be empty.
	IsDetached bool
}

// ResolveInput contains the parameters for slip resolution.
// The repository path is provided separately when creating the LocalGitRepository.
type ResolveInput struct {
	// Depth is the maximum number of commits to walk in the ancestry.
	// A higher value increases the chance of finding a matching slip
	// but also increases database query size.
	Depth int
}

// ResolveOutput contains the result of a successful slip resolution.
type ResolveOutput struct {
	// CorrelationID is the unique identifier of the resolved slip.
	// This is the primary output value written to stdout.
	CorrelationID string

	// MatchedCommit is the commit SHA that matched a slip in the database.
	// This may differ from the HEAD SHA if the slip was found in ancestry.
	MatchedCommit string

	// Repository is the repository name in owner/repo format.
	// Included for logging and verification purposes.
	Repository string

	// Branch is the branch name at resolution time (may be empty if detached).
	Branch string

	// ResolvedBy indicates how the slip was resolved.
	// Typically "ancestry" for this application.
	ResolvedBy string
}

// DefaultAncestryDepth is the default number of commits to walk when searching for slips.
const DefaultAncestryDepth = 25
