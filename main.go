// Package main is the entry point for the slippy-find CLI application.
// slippy-find resolves routing slips from local Git repository commit history,
// outputting only the correlation_id for consumption by external systems.
package main

import (
	"context"
	"os"
	"strings"

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
			// Defense-in-depth: a SLIPPY_API_URL that is set but contains
			// only whitespace silently falls through to K8S_NAMESPACE
			// resolution (slippyapi trims). Surface that as a WARN so
			// an operator-set blank value — or a templating bug emitting
			// a blank-but-present value — shows up rather than as silent
			// namespace-derived routing.
			if raw := os.Getenv("SLIPPY_API_URL"); raw != "" && strings.TrimSpace(raw) == "" {
				adapter.Warn(
					context.Background(),
					"SLIPPY_API_URL is set but contains only whitespace; falling through to K8S_NAMESPACE-based resolution",
					map[string]any{"k8s_namespace": os.Getenv("K8S_NAMESPACE")},
				)
			}
			cfg, err := config.Load()
			if err != nil {
				return nil, err
			}
			return &cmd.AppConfig{
				SlippyAPIURL: cfg.SlippyAPIURL,
				SlippyAPIKey: cfg.SlippyAPIKey,
				LogLevel:     cfg.LogLevel,
				LogAppName:   cfg.LogAppName,
			}, nil
		},

		GitRepoFactory: func(path string, _ cmd.Logger) (domain.LocalGitRepository, error) {
			return git.NewGoGitRepository(path, adapter)
		},

		SlipFinderFactory: func(cfg *cmd.AppConfig, _ cmd.Logger) (domain.SlipFinder, error) {
			return store.NewSlipAPIAdapter(cfg.SlippyAPIURL, cfg.SlippyAPIKey)
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
