// Package cmd provides the CLI commands for slippy-find.
package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/MyCarrier-DevOps/slippy-find/internal/domain"
)

// Logger defines the logging interface used by the command.
type Logger interface {
	Info(ctx context.Context, msg string, fields map[string]interface{})
	Debug(ctx context.Context, msg string, fields map[string]interface{})
	Warn(ctx context.Context, msg string, fields map[string]interface{})
	Error(ctx context.Context, msg string, err error, fields map[string]interface{})
}

// Dependencies holds all injectable dependencies for the command.
// This enables testing by allowing mock implementations to be injected.
type Dependencies struct {
	// LoggerFactory creates a logger instance.
	LoggerFactory func() Logger

	// ConfigLoader loads application configuration.
	ConfigLoader func() (*AppConfig, error)

	// GitRepoFactory creates a LocalGitRepository for the given path.
	GitRepoFactory func(path string, log Logger) (domain.LocalGitRepository, error)

	// SlipFinderFactory creates a SlipFinder using the given config.
	SlipFinderFactory func(cfg *AppConfig, log Logger) (domain.SlipFinder, error)

	// ResolverFactory creates a Resolver with the given dependencies.
	ResolverFactory func(
		gitRepo domain.LocalGitRepository,
		finder domain.SlipFinder,
		log Logger,
	) domain.Resolver

	// OutputWriterFactory creates an OutputWriter.
	OutputWriterFactory func() domain.OutputWriter

	// Stdout is the writer for standard output (for correlation ID).
	Stdout io.Writer

	// Stderr is the writer for standard error (for warnings/errors).
	Stderr io.Writer
}

// AppConfig holds application configuration loaded by ConfigLoader.
type AppConfig struct {
	// ClickHouseConfig is passed to the SlipFinderFactory.
	ClickHouseConfig any

	// PipelineConfig is passed to the SlipFinderFactory.
	PipelineConfig any

	// Database is the database name.
	Database string

	// LogLevel is the log level setting.
	LogLevel string

	// LogAppName is the application name for logging.
	LogAppName string
}

// Command-line flags.
var (
	depth   int
	verbose bool
)

// defaultDeps holds the production dependencies.
// This is set by the production wiring in main or via SetDefaultDependencies.
var defaultDeps *Dependencies

// SetDefaultDependencies sets the default dependencies for production use.
// This should be called from main() before Execute().
func SetDefaultDependencies(deps *Dependencies) {
	defaultDeps = deps
}

// NewRootCmd creates the root command for slippy-find.
func NewRootCmd() *cobra.Command {
	return NewRootCmdWithDeps(defaultDeps)
}

// NewRootCmdWithDeps creates the root command with explicit dependencies.
// This is the primary constructor that enables testing via dependency injection.
func NewRootCmdWithDeps(deps *Dependencies) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "slippy-find [path]",
		Short: "Resolve routing slips from local Git repository commit history",
		Long: `slippy-find resolves routing slips using local Git repository commit history.

It walks the commit ancestry from HEAD and queries the slip store to find
a matching routing slip. On success, it outputs only the correlation_id
to stdout for consumption by external systems.

All git context (HEAD SHA, branch, repository name) is derived from the
local repository. The repository name is extracted from the 'origin' remote URL.

Examples:
  # Resolve slip from current directory
  slippy-find

  # Resolve slip from a specific path
  slippy-find /path/to/repo

  # Increase ancestry search depth
  slippy-find --depth 50

  # Enable verbose logging
  slippy-find -v`,
		Args:         cobra.MaximumNArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runResolve(cmd, args, deps)
		},
	}

	// Define flags
	rootCmd.Flags().IntVarP(&depth, "depth", "d", domain.DefaultAncestryDepth,
		"Maximum ancestry depth to search for matching slips")
	rootCmd.Flags().BoolVarP(&verbose, "verbose", "v", false,
		"Enable verbose/debug logging")

	return rootCmd
}

// runResolve executes the slip resolution logic with injected dependencies.
func runResolve(cmd *cobra.Command, args []string, deps *Dependencies) error {
	if deps == nil {
		return errors.New("dependencies not configured")
	}

	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	// Determine repository path
	repoPath := "."
	if len(args) > 0 {
		repoPath = args[0]
	}

	// Get stderr for warnings
	stderr := deps.Stderr
	if stderr == nil {
		stderr = os.Stderr
	}

	// Set log level based on verbose flag (best-effort)
	if verbose {
		if err := os.Setenv("LOG_LEVEL", "debug"); err != nil {
			// Best-effort warning: ignore fprintf error as this is non-critical
			writeWarningf(stderr, "warning: could not set log level: %v\n", err)
		}
	}

	// Initialize logger
	log := deps.LoggerFactory()

	log.Info(ctx, "starting slippy-find", map[string]interface{}{
		"path":    repoPath,
		"depth":   depth,
		"verbose": verbose,
	})

	// Load configuration
	cfg, err := deps.ConfigLoader()
	if err != nil {
		log.Error(ctx, "failed to load configuration", err, nil)
		return fmt.Errorf("configuration error: %w", err)
	}

	// Initialize Git repository adapter
	gitRepo, err := deps.GitRepoFactory(repoPath, log)
	if err != nil {
		log.Error(ctx, "failed to open git repository", err, map[string]interface{}{
			"path": repoPath,
		})
		if errors.Is(err, domain.ErrRepositoryNotFound) {
			return fmt.Errorf("not a git repository: %s", repoPath)
		}
		return err
	}
	defer func() {
		if closeErr := gitRepo.Close(); closeErr != nil {
			log.Warn(ctx, "failed to close git repository", map[string]interface{}{
				"error": closeErr.Error(),
			})
		}
	}()

	// Initialize slip finder
	finder, err := deps.SlipFinderFactory(cfg, log)
	if err != nil {
		log.Error(ctx, "failed to initialize slip finder", err, nil)
		return fmt.Errorf("database error: %w", err)
	}
	defer func() {
		if closeErr := finder.Close(); closeErr != nil {
			log.Warn(ctx, "failed to close slip finder", map[string]interface{}{
				"error": closeErr.Error(),
			})
		}
	}()

	// Create resolver and resolve slip
	resolver := deps.ResolverFactory(gitRepo, finder, log)
	result, err := resolver.Resolve(ctx, domain.ResolveInput{
		Depth: depth,
	})
	if err != nil {
		log.Error(ctx, "failed to resolve slip", err, nil)
		if errors.Is(err, domain.ErrNoAncestorSlip) {
			return fmt.Errorf("no slip found in commit ancestry")
		}
		if errors.Is(err, domain.ErrNoRemoteOrigin) {
			return fmt.Errorf("no 'origin' remote configured; cannot determine repository name")
		}
		return err
	}

	// Write correlation ID to stdout
	writer := deps.OutputWriterFactory()
	if err := writer.WriteCorrelationID(result.CorrelationID); err != nil {
		log.Error(ctx, "failed to write output", err, nil)
		return fmt.Errorf("output error: %w", err)
	}

	log.Info(ctx, "slip resolution complete", map[string]interface{}{
		"correlation_id": result.CorrelationID,
		"matched_commit": result.MatchedCommit,
		"repository":     result.Repository,
		"resolved_by":    result.ResolvedBy,
	})

	return nil
}

// Execute runs the root command.
func Execute() {
	rootCmd := NewRootCmd()
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// writeWarningf writes a warning message to the given writer.
// This is a best-effort operation; errors are intentionally ignored
// because there is no recovery action if stderr writes fail.
func writeWarningf(w io.Writer, format string, args ...any) {
	_, err := fmt.Fprintf(w, format, args...)
	if err != nil {
		// Intentionally ignored: no recovery action for failed stderr writes
		return
	}
}
