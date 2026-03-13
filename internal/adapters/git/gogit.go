// Package git provides adapters for interacting with local Git repositories.
// This package implements the domain.LocalGitRepository interface using go-git/v5.
package git

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"

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

// GetCommitAncestry walks the first-parent chain from HEAD, returning commit SHAs.
// Returns commits in order from newest (HEAD) to oldest, up to depth commits.
//
// Only the first parent of each commit is followed. This prevents merge commits
// from polluting ancestry with commits from other branches (e.g., merging main
// into a feature branch would otherwise include main's commits, causing
// incorrect slip resolution).
//
// Special case: when HEAD is a detached merge commit (typical of GitHub Actions
// pull_request checkout), all parent chains are walked independently. This
// ensures both the base branch and feature branch commits are searched, since
// the routing slip is typically associated with a feature branch commit.
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
	headCommit, err := r.repo.CommitObject(head.Hash())
	if err != nil {
		return nil, fmt.Errorf("failed to get commit object for HEAD: %w", err)
	}

	seen := make(map[string]bool)
	var commits []string

	// When HEAD is a detached merge commit (e.g., CI merge commit created by
	// actions/checkout for pull requests), walk all parent chains. The first
	// parent is the base branch and subsequent parents are the merged-in
	// branches. Without this, we'd only search the base branch and miss the
	// feature branch commits where the routing slip was created.
	if headCommit.NumParents() > 1 && !head.Name().IsBranch() {
		r.logger.Debug(ctx, "HEAD is a detached merge commit; walking all parent chains", map[string]interface{}{
			"num_parents": headCommit.NumParents(),
			"head_sha":    headCommit.Hash.String(),
		})

		// Include the merge commit itself
		commits = append(commits, headCommit.Hash.String())
		seen[headCommit.Hash.String()] = true

		// Walk each parent chain with full depth budget
		for i := range headCommit.NumParents() {
			parent, pErr := headCommit.Parent(i)
			if pErr != nil {
				continue
			}
			if err := r.walkFirstParent(ctx, parent, depth, seen, &commits); err != nil {
				return nil, err
			}
		}
	} else {
		// Normal case: walk first-parent chain only from HEAD
		if err := r.walkFirstParent(ctx, headCommit, depth, seen, &commits); err != nil {
			return nil, err
		}
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

// walkFirstParent walks the first-parent chain from start, appending at most
// limit unseen commit SHAs to commits. The seen map is used to deduplicate
// across multiple walks (e.g., when parent chains converge).
func (r *GoGitRepository) walkFirstParent(
	ctx context.Context,
	start *object.Commit,
	limit int,
	seen map[string]bool,
	commits *[]string,
) error {
	current := start
	walked := 0
	for walked < limit {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		sha := current.Hash.String()
		if seen[sha] {
			break
		}
		seen[sha] = true
		*commits = append(*commits, sha)
		walked++

		if current.NumParents() == 0 {
			break
		}
		parent, err := current.Parent(0)
		if err != nil {
			return fmt.Errorf("failed to get parent commit: %w", err)
		}
		current = parent
	}
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
