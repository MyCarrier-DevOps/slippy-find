// Package main is the entry point for the slippy-find CLI application.
// slippy-find resolves routing slips from local Git repository commit history,
// outputting only the correlation_id for consumption by external systems.
package main

import (
	"context"
	"os"
	"time"

	"github.com/MyCarrier-DevOps/goLibMyCarrier/logger"

	"github.com/MyCarrier-DevOps/slippy-find/cmd"
	"github.com/MyCarrier-DevOps/slippy-find/internal/adapters/git"
	logadapter "github.com/MyCarrier-DevOps/slippy-find/internal/adapters/logger"
	"github.com/MyCarrier-DevOps/slippy-find/internal/adapters/output"
	"github.com/MyCarrier-DevOps/slippy-find/internal/adapters/store"
	"github.com/MyCarrier-DevOps/slippy-find/internal/domain"
	"github.com/MyCarrier-DevOps/slippy-find/internal/infrastructure/config"
	"github.com/MyCarrier-DevOps/slippy-find/internal/usecases"
)

func main() {
	zapLog := logger.NewZapLoggerFromConfig()
	adapter := logadapter.NewZapAdapter(zapLog)

	deps := &cmd.Dependencies{
		LoggerFactory: func() cmd.Logger {
			return adapter
		},

		ConfigLoader: func() (*cmd.AppConfig, error) {
			cfg, err := config.Load()
			if err != nil {
				return nil, err
			}
			return &cmd.AppConfig{
				SlippyAPIURL:      cfg.SlippyAPIURL,
				SlippyAPIKey:      cfg.SlippyAPIKey,
				SlippyAPIIPv4Only: cfg.SlippyAPIIPv4Only,
				LogLevel:          cfg.LogLevel,
				LogAppName:        cfg.LogAppName,
			}, nil
		},

		GitRepoFactory: func(path string, _ cmd.Logger) (domain.LocalGitRepository, error) {
			return git.NewGoGitRepository(path, adapter)
		},

		SlipFinderFactory: func(cfg *cmd.AppConfig, _ cmd.Logger) (domain.SlipFinder, error) {
			return store.NewSlipAPIAdapter(cfg.SlippyAPIURL, cfg.SlippyAPIKey,
				store.WithIPv4Only(cfg.SlippyAPIIPv4Only),
				store.WithRetryNotifier(func(
					ctx context.Context, attempt int, delay time.Duration, err error,
				) {
					adapter.Warn(ctx, "slippy-api call failed, retrying", map[string]interface{}{
						"attempt":  attempt,
						"retry_in": delay.String(),
						"error":    err.Error(),
					})
				}),
			)
		},

		ResolverFactory: func(
			gitRepo domain.LocalGitRepository,
			finder domain.SlipFinder,
			_ cmd.Logger,
		) domain.Resolver {
			return usecases.NewSlipResolver(gitRepo, finder, adapter)
		},

		OutputWriterFactory: func() domain.OutputWriter {
			return output.NewWriter()
		},

		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}

	cmd.SetDefaultDependencies(deps)
	cmd.Execute()
}
