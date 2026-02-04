// Package main is the entry point for the slippy-find CLI application.
// slippy-find resolves routing slips from local Git repository commit history,
// outputting only the correlation_id for consumption by external systems.
package main

import (
	"os"

	ch "github.com/MyCarrier-DevOps/goLibMyCarrier/clickhouse"
	"github.com/MyCarrier-DevOps/goLibMyCarrier/logger"
	"github.com/MyCarrier-DevOps/goLibMyCarrier/slippy"

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
	// Create a single shared logger instance for the application
	zapLog := logger.NewZapLoggerFromConfig()
	adapter := logadapter.NewZapAdapter(zapLog)

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
			return git.NewGoGitRepository(path, adapter)
		},

		SlipFinderFactory: func(cfg *cmd.AppConfig, _ cmd.Logger) (domain.SlipFinder, error) {
			chConfig, ok := cfg.ClickHouseConfig.(*ch.ClickhouseConfig)
			if !ok {
				return nil, newConfigTypeError("*ch.ClickhouseConfig")
			}

			pipelineCfg, ok := cfg.PipelineConfig.(*slippy.PipelineConfig)
			if !ok {
				return nil, newConfigTypeError("*slippy.PipelineConfig")
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

func newConfigTypeError(expected string) error {
	return &configTypeError{expected: expected}
}

// configTypeError is returned when configuration type assertion fails.
type configTypeError struct {
	expected string
}

func (e *configTypeError) Error() string {
	return "invalid configuration type: expected " + e.expected
}
