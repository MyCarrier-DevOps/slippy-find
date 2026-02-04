// Package domain defines the core business entities and interfaces for slippy-find.
// This package contains no external dependencies and represents the innermost layer
// of the CLEAN architecture.
package domain

import (
	"context"
	"errors"
)

// Domain errors for git operations and slip resolution.
var (
	// ErrRepositoryNotFound indicates the specified path is not a valid Git repository.
	ErrRepositoryNotFound = errors.New("git repository not found at specified path")

	// ErrNoRemoteOrigin indicates no 'origin' remote is configured in the repository.
	ErrNoRemoteOrigin = errors.New("no 'origin' remote configured; cannot determine repository name")

	// ErrInvalidRemoteURL indicates the remote URL could not be parsed to extract owner/repo.
	ErrInvalidRemoteURL = errors.New("could not parse repository name from remote URL")

	// ErrNoAncestorSlip indicates no slip was found in the commit ancestry.
	ErrNoAncestorSlip = errors.New("no slip found in commit ancestry")

	// ErrEmptyAncestry indicates the commit ancestry walk returned no commits.
	ErrEmptyAncestry = errors.New("commit ancestry is empty")
)

// LocalGitRepository provides git context and commit ancestry from a local repository.
// This interface replaces the GitHub API-based GitHubAPI interface from goLibMyCarrier/slippy.
// The repository path is the ONLY external input - all other context is derived from Git.
type LocalGitRepository interface {
	// GetGitContext extracts all necessary context from the repository.
	// This includes HEAD SHA, branch name, and repository name derived from origin remote.
	// Returns ErrNoRemoteOrigin if no origin remote is configured.
	// Logs a warning if HEAD is detached but continues with empty branch name.
	GetGitContext(ctx context.Context) (*GitContext, error)

	// GetCommitAncestry walks the commit graph from HEAD, returning commit SHAs.
	// Returns commits in order from newest (HEAD) to oldest, up to depth commits.
	// The depth parameter limits how far back in history to walk.
	GetCommitAncestry(ctx context.Context, depth int) ([]string, error)

	// Close releases any resources held by the repository.
	Close() error
}

// OutputWriter writes resolved slip data to an output destination.
type OutputWriter interface {
	// WriteCorrelationID writes the correlation ID to the output.
	WriteCorrelationID(correlationID string) error
}

// SlipFinder queries the slip store to find slips by commit ancestry.
type SlipFinder interface {
	// FindByCommits searches for a slip matching any of the given commits.
	// Returns the slip, the matched commit SHA, and any error.
	// Returns (nil, "", nil) if no matching slip is found.
	FindByCommits(ctx context.Context, repository string, commits []string) (*Slip, string, error)

	// Close releases any resources held by the finder.
	Close() error
}

// Slip represents a routing slip found in the store.
// This is a domain representation - the actual slip structure comes from goLibMyCarrier.
type Slip struct {
	// CorrelationID is the unique identifier for the slip.
	CorrelationID string
}

// Resolver resolves routing slips from git context.
type Resolver interface {
	// Resolve finds a routing slip for the current git state.
	Resolve(ctx context.Context, input ResolveInput) (*ResolveOutput, error)
}
