// Package usecases contains the application business logic.
// This package orchestrates domain entities and interfaces to fulfill use cases.
package usecases

import (
	"context"
	"fmt"

	"github.com/MyCarrier-DevOps/slippy-find/internal/domain"
)

// Logger defines the logging interface required by the resolver.
// This abstracts the logger dependency to avoid coupling to a specific implementation.
type Logger interface {
	Info(ctx context.Context, msg string, fields map[string]interface{})
	Debug(ctx context.Context, msg string, fields map[string]interface{})
	Warn(ctx context.Context, msg string, fields map[string]interface{})
	Error(ctx context.Context, msg string, err error, fields map[string]interface{})
}

// SlipResolver resolves routing slips from local Git repository commit ancestry.
// It implements the core business logic for finding the correlation_id of a slip
// that matches commits in the local repository's history.
type SlipResolver struct {
	gitRepo domain.LocalGitRepository
	finder  domain.SlipFinder
	logger  Logger
}

// NewSlipResolver creates a new SlipResolver with the given dependencies.
// All dependencies are injected to support testing and SOLID principles.
func NewSlipResolver(
	gitRepo domain.LocalGitRepository,
	finder domain.SlipFinder,
	log Logger,
) *SlipResolver {
	return &SlipResolver{
		gitRepo: gitRepo,
		finder:  finder,
		logger:  log,
	}
}

// Resolve finds the routing slip that matches the local repository's commit ancestry.
// It walks the commit history from HEAD up to the specified depth and queries
// the SlipStore to find a matching slip.
//
// Returns the ResolveOutput containing the correlation_id and match details,
// or an error if no slip is found or an operation fails.
func (r *SlipResolver) Resolve(ctx context.Context, input domain.ResolveInput) (*domain.ResolveOutput, error) {
	// Apply default depth if not specified
	depth := input.Depth
	if depth <= 0 {
		depth = domain.DefaultAncestryDepth
	}

	r.logger.Info(ctx, "starting slip resolution", map[string]interface{}{
		"depth": depth,
	})

	// Get git context (HEAD SHA, branch, repository name)
	gitCtx, err := r.gitRepo.GetGitContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get git context: %w", err)
	}

	r.logger.Info(ctx, "extracted git context", map[string]interface{}{
		"repository":  gitCtx.Repository,
		"branch":      gitCtx.Branch,
		"head_sha":    gitCtx.HeadSHA,
		"is_detached": gitCtx.IsDetached,
	})

	// Get commit ancestry from HEAD
	commits, err := r.gitRepo.GetCommitAncestry(ctx, depth)
	if err != nil {
		return nil, fmt.Errorf("failed to get commit ancestry: %w", err)
	}

	r.logger.Debug(ctx, "retrieved commit ancestry", map[string]interface{}{
		"repository":    gitCtx.Repository,
		"commits_count": len(commits),
		"head":          commits[0],
	})

	// Find slip matching any commit in ancestry
	foundSlip, matchedCommit, err := r.finder.FindByCommits(ctx, gitCtx.Repository, commits)
	if err != nil {
		return nil, fmt.Errorf("failed to find slip by commits: %w", err)
	}

	if foundSlip == nil {
		r.logger.Warn(ctx, "no slip found in commit ancestry", map[string]interface{}{
			"repository":    gitCtx.Repository,
			"commits_count": len(commits),
			"head_sha":      gitCtx.HeadSHA,
		})
		return nil, fmt.Errorf(
			"%w: searched %d commits from %s",
			domain.ErrNoAncestorSlip,
			len(commits),
			gitCtx.HeadSHA,
		)
	}

	r.logger.Info(ctx, "slip resolved successfully", map[string]interface{}{
		"correlation_id": foundSlip.CorrelationID,
		"matched_commit": matchedCommit,
		"repository":     gitCtx.Repository,
		"resolved_by":    "ancestry",
	})

	return &domain.ResolveOutput{
		CorrelationID: foundSlip.CorrelationID,
		MatchedCommit: matchedCommit,
		Repository:    gitCtx.Repository,
		Branch:        gitCtx.Branch,
		ResolvedBy:    "ancestry",
	}, nil
}
