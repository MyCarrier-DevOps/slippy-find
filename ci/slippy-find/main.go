// Package main is the entry point for the slippy-find CLI application.
// slippy-find resolves routing slips from local Git repository commit history,
// outputting only the correlation_id for consumption by external systems.
package main

import (
	"context"
	"os"

	ch "github.com/MyCarrier-DevOps/goLibMyCarrier/clickhouse"
	"github.com/MyCarrier-DevOps/goLibMyCarrier/logger"
	"github.com/MyCarrier-DevOps/goLibMyCarrier/slippy"

	"github.com/MyCarrier-DevOps/slippy-find/ci/slippy-find/cmd"
	"github.com/MyCarrier-DevOps/slippy-find/ci/slippy-find/internal/adapters/git"
	"github.com/MyCarrier-DevOps/slippy-find/ci/slippy-find/internal/adapters/output"
	"github.com/MyCarrier-DevOps/slippy-find/ci/slippy-find/internal/adapters/store"
	"github.com/MyCarrier-DevOps/slippy-find/ci/slippy-find/internal/domain"
	"github.com/MyCarrier-DevOps/slippy-find/ci/slippy-find/internal/infrastructure/config"
	"github.com/MyCarrier-DevOps/slippy-find/ci/slippy-find/internal/usecases"
)

// loggerAdapter adapts logger.Logger to the various logger interfaces.
type loggerAdapter struct {
	log logger.Logger
}

func (a *loggerAdapter) Info(ctx context.Context, msg string, fields map[string]interface{}) {
	a.log.Info(ctx, msg, fields)
}

func (a *loggerAdapter) Debug(ctx context.Context, msg string, fields map[string]interface{}) {
	a.log.Debug(ctx, msg, fields)
}

func (a *loggerAdapter) Warn(ctx context.Context, msg string, fields map[string]interface{}) {
	a.log.Warn(ctx, msg, fields)
}

func (a *loggerAdapter) Error(ctx context.Context, msg string, err error, fields map[string]interface{}) {
	a.log.Error(ctx, msg, err, fields)
}

func main() {
	// Create a single shared logger instance for the application
	zapLog := logger.NewZapLoggerFromConfig()
	adapter := &loggerAdapter{log: zapLog}

	// Wire up production dependencies
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
				ClickHouseConfig: cfg.ClickHouse,
				PipelineConfig:   cfg.PipelineConfig,
				Database:         cfg.Database,
				LogLevel:         cfg.LogLevel,
				LogAppName:       cfg.LogAppName,
			}, nil
		},

		GitRepoFactory: func(path string, _ cmd.Logger) (domain.LocalGitRepository, error) {
			// Use the shared adapter for the git repository
			return git.NewGoGitRepository(path, adapter)
		},

		SlipFinderFactory: func(cfg *cmd.AppConfig, _ cmd.Logger) (domain.SlipFinder, error) {
			// The ClickHouseConfig comes from goLibMyCarrier/clickhouse package
			chConfig, ok := cfg.ClickHouseConfig.(*ch.ClickhouseConfig)
			if !ok {
				return nil, &configTypeError{expected: "*ch.ClickhouseConfig", got: cfg.ClickHouseConfig}
			}

			pipelineCfg, ok := cfg.PipelineConfig.(*slippy.PipelineConfig)
			if !ok {
				return nil, &configTypeError{expected: "*slippy.PipelineConfig", got: cfg.PipelineConfig}
			}

			slippyStore, err := slippy.NewClickHouseStoreFromConfig(chConfig, slippy.ClickHouseStoreOptions{
				PipelineConfig: pipelineCfg,
				Database:       cfg.Database,
				Logger:         zapLog,
				SkipMigrations: true,
			})
			if err != nil {
				return nil, err
			}
			return store.NewClickHouseAdapter(slippyStore), nil
		},

		ResolverFactory: func(
			gitRepo domain.LocalGitRepository,
			finder domain.SlipFinder,
			_ cmd.Logger,
		) domain.Resolver {
			// Use the shared adapter for the resolver
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

// configTypeError is returned when configuration type assertion fails.
type configTypeError struct {
	expected string
	got      interface{}
}

func (e *configTypeError) Error() string {
	return "invalid configuration type: expected " + e.expected
}
