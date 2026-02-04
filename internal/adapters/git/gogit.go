// Package git provides adapters for interacting with local Git repositories.
// This package implements the domain.LocalGitRepository interface using go-git/v5.
package git

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/storer"

	"github.com/MyCarrier-DevOps/slippy-find/internal/domain"
)

// Logger defines the logging interface for the git adapter.
// This interface enables dependency injection and testability.
type Logger interface {
	Debug(ctx context.Context, msg string, fields map[string]interface{})
	Warn(ctx context.Context, msg string, fields map[string]interface{})
}

// GoGitRepository implements domain.LocalGitRepository using go-git/v5.
// It provides local Git repository operations for commit ancestry resolution.
type GoGitRepository struct {
	repo   *git.Repository
	path   string
	logger Logger
}

// NewGoGitRepository creates a new GoGitRepository for the given path.
// The path can be either a working directory or a bare repository.
// Returns domain.ErrRepositoryNotFound if the path is not a valid Git repository.
func NewGoGitRepository(path string, log Logger) (*GoGitRepository, error) {
	repo, err := git.PlainOpen(path)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", domain.ErrRepositoryNotFound, path)
	}

	return &GoGitRepository{
		repo:   repo,
		path:   path,
		logger: log,
	}, nil
}

// GetGitContext extracts all necessary context from the repository.
// Returns GitContext with HEAD SHA, branch name, and repository name.
// Logs a warning if HEAD is detached but continues with empty branch name.
// Returns domain.ErrNoRemoteOrigin if no origin remote is configured.
func (r *GoGitRepository) GetGitContext(ctx context.Context) (*domain.GitContext, error) {
	// Get HEAD reference
	head, err := r.repo.Head()
	if err != nil {
		return nil, fmt.Errorf("failed to get HEAD: %w", err)
	}

	gitCtx := &domain.GitContext{
		HeadSHA:    head.Hash().String(),
		IsDetached: !head.Name().IsBranch(),
	}

	// Get branch name if on a branch
	if head.Name().IsBranch() {
		gitCtx.Branch = head.Name().Short()
	} else {
		// HEAD is detached - warn but continue
		r.logger.Warn(ctx, "HEAD is detached; branch name will be empty", map[string]interface{}{
			"head_sha": gitCtx.HeadSHA,
			"path":     r.path,
		})
	}

	// Get repository name from origin remote
	remote, err := r.repo.Remote("origin")
	if err != nil {
		return nil, fmt.Errorf("%w: failed to get origin remote: %w", domain.ErrNoRemoteOrigin, err)
	}

	urls := remote.Config().URLs
	if len(urls) == 0 {
		return nil, fmt.Errorf("%w: origin remote has no URLs configured", domain.ErrNoRemoteOrigin)
	}

	repoName, err := parseRepoFromURL(urls[0])
	if err != nil {
		return nil, fmt.Errorf("%w: failed to parse URL: %w", domain.ErrInvalidRemoteURL, err)
	}
	gitCtx.Repository = repoName

	r.logger.Debug(ctx, "extracted git context", map[string]interface{}{
		"head_sha":    gitCtx.HeadSHA,
		"branch":      gitCtx.Branch,
		"repository":  gitCtx.Repository,
		"is_detached": gitCtx.IsDetached,
	})

	return gitCtx, nil
}

// GetCommitAncestry walks the commit graph from HEAD, returning commit SHAs.
// Returns commits in order from newest (HEAD) to oldest, up to depth commits.
func (r *GoGitRepository) GetCommitAncestry(ctx context.Context, depth int) ([]string, error) {
	if depth <= 0 {
		depth = domain.DefaultAncestryDepth
	}

	// Get HEAD reference
	head, err := r.repo.Head()
	if err != nil {
		return nil, fmt.Errorf("failed to get HEAD: %w", err)
	}

	// Get the commit object for HEAD
	commit, err := r.repo.CommitObject(head.Hash())
	if err != nil {
		return nil, fmt.Errorf("failed to get commit object for HEAD: %w", err)
	}

	// Walk commit history from HEAD using commit-time ordering
	var commits []string
	iter := object.NewCommitIterCTime(commit, nil, nil)

	err = iter.ForEach(func(c *object.Commit) error {
		// Check context for cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if len(commits) >= depth {
			return storer.ErrStop
		}
		commits = append(commits, c.Hash.String())
		return nil
	})

	// ErrStop is expected when we reach depth limit
	if err != nil && !errors.Is(err, storer.ErrStop) {
		return nil, fmt.Errorf("failed to walk commit history: %w", err)
	}

	if len(commits) == 0 {
		return nil, domain.ErrEmptyAncestry
	}

	r.logger.Debug(ctx, "walked commit ancestry", map[string]interface{}{
		"depth_requested": depth,
		"commits_found":   len(commits),
		"head_sha":        commits[0],
		"oldest_sha":      commits[len(commits)-1],
	})

	return commits, nil
}

// Close releases any resources held by the repository.
// For go-git, this is a no-op as the repository doesn't hold persistent resources.
func (r *GoGitRepository) Close() error {
	return nil
}

// Regular expressions for parsing Git remote URLs.
var (
	// httpsURLPattern matches HTTPS URLs like:
	// https://github.com/owner/repo.git
	// https://github.com/owner/repo
	httpsURLPattern = regexp.MustCompile(`^https?://[^/]+/([^/]+)/([^/]+?)(?:\.git)?$`)

	// sshURLPattern matches SSH URLs like:
	// git@github.com:owner/repo.git
	// git@github.com:owner/repo
	sshURLPattern = regexp.MustCompile(`^git@[^:]+:([^/]+)/([^/]+?)(?:\.git)?$`)
)

// parseRepoFromURL extracts owner/repo from a Git remote URL.
// Supports both HTTPS and SSH formats:
//   - https://github.com/owner/repo.git -> owner/repo
//   - https://github.com/owner/repo -> owner/repo
//   - git@github.com:owner/repo.git -> owner/repo
//   - git@github.com:owner/repo -> owner/repo
func parseRepoFromURL(url string) (string, error) {
	url = strings.TrimSpace(url)

	// Try HTTPS pattern first
	if matches := httpsURLPattern.FindStringSubmatch(url); len(matches) == 3 {
		return matches[1] + "/" + matches[2], nil
	}

	// Try SSH pattern
	if matches := sshURLPattern.FindStringSubmatch(url); len(matches) == 3 {
		return matches[1] + "/" + matches[2], nil
	}

	return "", fmt.Errorf("unrecognized URL format: %s", url)
}
