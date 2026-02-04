// Package cmd provides CLI commands for slippy-find.
package cmd

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/MyCarrier-DevOps/slippy-find/ci/slippy-find/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test mocks for dependency injection testing.

// mockLogger implements the Logger interface for testing.
type mockLogger struct{}

func (m *mockLogger) Info(_ context.Context, _ string, _ map[string]interface{})           {}
func (m *mockLogger) Debug(_ context.Context, _ string, _ map[string]interface{})          {}
func (m *mockLogger) Warn(_ context.Context, _ string, _ map[string]interface{})           {}
func (m *mockLogger) Error(_ context.Context, _ string, _ error, _ map[string]interface{}) {}

// mockGitRepo implements domain.LocalGitRepository for testing.
type mockGitRepo struct {
	gitContext  *domain.GitContext
	gitCtxErr   error
	commits     []string
	commitsErr  error
	closeErr    error
	closeCalled bool
}

func (m *mockGitRepo) GetGitContext(_ context.Context) (*domain.GitContext, error) {
	return m.gitContext, m.gitCtxErr
}

func (m *mockGitRepo) GetCommitAncestry(_ context.Context, _ int) ([]string, error) {
	return m.commits, m.commitsErr
}

func (m *mockGitRepo) Close() error {
	m.closeCalled = true
	return m.closeErr
}

// mockSlipFinder implements domain.SlipFinder for testing.
type mockSlipFinder struct {
	slip        *domain.Slip
	matchCommit string
	findErr     error
	closeErr    error
	closeCalled bool
}

func (m *mockSlipFinder) FindByCommits(_ context.Context, _ string, _ []string) (*domain.Slip, string, error) {
	return m.slip, m.matchCommit, m.findErr
}

func (m *mockSlipFinder) Close() error {
	m.closeCalled = true
	return m.closeErr
}

// mockResolver implements domain.Resolver for testing.
type mockResolver struct {
	output *domain.ResolveOutput
	err    error
}

func (m *mockResolver) Resolve(_ context.Context, _ domain.ResolveInput) (*domain.ResolveOutput, error) {
	return m.output, m.err
}

// mockOutputWriter implements domain.OutputWriter for testing.
type mockOutputWriter struct {
	writtenID string
	writeErr  error
}

func (m *mockOutputWriter) WriteCorrelationID(id string) error {
	m.writtenID = id
	return m.writeErr
}

func TestNewRootCmd(t *testing.T) {
	// Set default deps so NewRootCmd() works
	SetDefaultDependencies(&Dependencies{})
	cmd := NewRootCmd()

	require.NotNil(t, cmd)
	assert.Equal(t, "slippy-find [path]", cmd.Use)
	assert.NotEmpty(t, cmd.Short)
	assert.NotEmpty(t, cmd.Long)
	assert.True(t, cmd.SilenceUsage)

	// Check flags are registered
	depthFlag := cmd.Flags().Lookup("depth")
	require.NotNil(t, depthFlag)
	assert.Equal(t, "d", depthFlag.Shorthand)
	assert.Equal(t, "25", depthFlag.DefValue)

	verboseFlag := cmd.Flags().Lookup("verbose")
	require.NotNil(t, verboseFlag)
	assert.Equal(t, "v", verboseFlag.Shorthand)
	assert.Equal(t, "false", verboseFlag.DefValue)
}

func TestNewRootCmd_MaxArgs(t *testing.T) {
	SetDefaultDependencies(&Dependencies{})
	cmd := NewRootCmd()

	// Test with no args - should be allowed
	err := cmd.Args(cmd, []string{})
	require.NoError(t, err)

	// Test with one arg - should be allowed
	err = cmd.Args(cmd, []string{"/path/to/repo"})
	require.NoError(t, err)

	// Test with two args - should fail
	err = cmd.Args(cmd, []string{"/path/one", "/path/two"})
	require.Error(t, err)
}

func TestNewRootCmd_HelpOutput(t *testing.T) {
	SetDefaultDependencies(&Dependencies{})
	cmd := NewRootCmd()

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"--help"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "slippy-find")
	assert.Contains(t, output, "--depth")
	assert.Contains(t, output, "--verbose")
}

func TestRootCmd_NilDependencies(t *testing.T) {
	cmd := NewRootCmdWithDeps(nil)
	cmd.SetArgs([]string{"."})

	err := cmd.Execute()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "dependencies not configured")
}

func TestRootCmd_ConfigLoadError(t *testing.T) {
	deps := &Dependencies{
		LoggerFactory: func() Logger { return &mockLogger{} },
		ConfigLoader: func() (*AppConfig, error) {
			return nil, errors.New("failed to load config")
		},
		Stderr: io.Discard,
	}

	cmd := NewRootCmdWithDeps(deps)
	cmd.SetArgs([]string{"."})

	err := cmd.Execute()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "configuration error")
}

func TestRootCmd_GitRepoError(t *testing.T) {
	deps := &Dependencies{
		LoggerFactory: func() Logger { return &mockLogger{} },
		ConfigLoader: func() (*AppConfig, error) {
			return &AppConfig{Database: "ci"}, nil
		},
		GitRepoFactory: func(_ string, _ Logger) (domain.LocalGitRepository, error) {
			return nil, domain.ErrRepositoryNotFound
		},
		Stderr: io.Discard,
	}

	cmd := NewRootCmdWithDeps(deps)
	cmd.SetArgs([]string{"/tmp/not-a-repo"})

	err := cmd.Execute()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a git repository")
}

func TestRootCmd_SlipFinderError(t *testing.T) {
	mockGit := &mockGitRepo{}
	deps := &Dependencies{
		LoggerFactory: func() Logger { return &mockLogger{} },
		ConfigLoader: func() (*AppConfig, error) {
			return &AppConfig{Database: "ci"}, nil
		},
		GitRepoFactory: func(_ string, _ Logger) (domain.LocalGitRepository, error) {
			return mockGit, nil
		},
		SlipFinderFactory: func(_ *AppConfig, _ Logger) (domain.SlipFinder, error) {
			return nil, errors.New("database connection failed")
		},
		Stderr: io.Discard,
	}

	cmd := NewRootCmdWithDeps(deps)
	cmd.SetArgs([]string{"."})

	err := cmd.Execute()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "database error")
	assert.True(t, mockGit.closeCalled, "git repo should be closed on error")
}

func TestRootCmd_ResolveError_NoSlipFound(t *testing.T) {
	mockGit := &mockGitRepo{}
	mockFinder := &mockSlipFinder{}

	deps := &Dependencies{
		LoggerFactory: func() Logger { return &mockLogger{} },
		ConfigLoader: func() (*AppConfig, error) {
			return &AppConfig{Database: "ci"}, nil
		},
		GitRepoFactory: func(_ string, _ Logger) (domain.LocalGitRepository, error) {
			return mockGit, nil
		},
		SlipFinderFactory: func(_ *AppConfig, _ Logger) (domain.SlipFinder, error) {
			return mockFinder, nil
		},
		ResolverFactory: func(_ domain.LocalGitRepository, _ domain.SlipFinder, _ Logger) domain.Resolver {
			return &mockResolver{err: domain.ErrNoAncestorSlip}
		},
		Stderr: io.Discard,
	}

	cmd := NewRootCmdWithDeps(deps)
	cmd.SetArgs([]string{"."})

	err := cmd.Execute()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no slip found")
	assert.True(t, mockGit.closeCalled)
	assert.True(t, mockFinder.closeCalled)
}

func TestRootCmd_ResolveError_NoOrigin(t *testing.T) {
	mockGit := &mockGitRepo{}
	mockFinder := &mockSlipFinder{}

	deps := &Dependencies{
		LoggerFactory: func() Logger { return &mockLogger{} },
		ConfigLoader: func() (*AppConfig, error) {
			return &AppConfig{Database: "ci"}, nil
		},
		GitRepoFactory: func(_ string, _ Logger) (domain.LocalGitRepository, error) {
			return mockGit, nil
		},
		SlipFinderFactory: func(_ *AppConfig, _ Logger) (domain.SlipFinder, error) {
			return mockFinder, nil
		},
		ResolverFactory: func(_ domain.LocalGitRepository, _ domain.SlipFinder, _ Logger) domain.Resolver {
			return &mockResolver{err: domain.ErrNoRemoteOrigin}
		},
		Stderr: io.Discard,
	}

	cmd := NewRootCmdWithDeps(deps)
	cmd.SetArgs([]string{"."})

	err := cmd.Execute()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no 'origin' remote configured")
}

func TestRootCmd_OutputWriteError(t *testing.T) {
	mockGit := &mockGitRepo{}
	mockFinder := &mockSlipFinder{}
	mockWriter := &mockOutputWriter{writeErr: errors.New("write failed")}

	deps := &Dependencies{
		LoggerFactory: func() Logger { return &mockLogger{} },
		ConfigLoader: func() (*AppConfig, error) {
			return &AppConfig{Database: "ci"}, nil
		},
		GitRepoFactory: func(_ string, _ Logger) (domain.LocalGitRepository, error) {
			return mockGit, nil
		},
		SlipFinderFactory: func(_ *AppConfig, _ Logger) (domain.SlipFinder, error) {
			return mockFinder, nil
		},
		ResolverFactory: func(_ domain.LocalGitRepository, _ domain.SlipFinder, _ Logger) domain.Resolver {
			return &mockResolver{
				output: &domain.ResolveOutput{
					CorrelationID: "test-id",
					MatchedCommit: "abc123",
					Repository:    "test/repo",
				},
			}
		},
		OutputWriterFactory: func() domain.OutputWriter {
			return mockWriter
		},
		Stderr: io.Discard,
	}

	cmd := NewRootCmdWithDeps(deps)
	cmd.SetArgs([]string{"."})

	err := cmd.Execute()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "output error")
}

func TestRootCmd_Success(t *testing.T) {
	mockGit := &mockGitRepo{}
	mockFinder := &mockSlipFinder{}
	mockWriter := &mockOutputWriter{}

	deps := &Dependencies{
		LoggerFactory: func() Logger { return &mockLogger{} },
		ConfigLoader: func() (*AppConfig, error) {
			return &AppConfig{Database: "ci"}, nil
		},
		GitRepoFactory: func(_ string, _ Logger) (domain.LocalGitRepository, error) {
			return mockGit, nil
		},
		SlipFinderFactory: func(_ *AppConfig, _ Logger) (domain.SlipFinder, error) {
			return mockFinder, nil
		},
		ResolverFactory: func(_ domain.LocalGitRepository, _ domain.SlipFinder, _ Logger) domain.Resolver {
			return &mockResolver{
				output: &domain.ResolveOutput{
					CorrelationID: "test-correlation-id-123",
					MatchedCommit: "abc123def456",
					Repository:    "MyCarrier-DevOps/test-repo",
					Branch:        "main",
					ResolvedBy:    "ancestry",
				},
			}
		},
		OutputWriterFactory: func() domain.OutputWriter {
			return mockWriter
		},
		Stdout: io.Discard,
		Stderr: io.Discard,
	}

	cmd := NewRootCmdWithDeps(deps)
	cmd.SetArgs([]string{"."})

	err := cmd.Execute()

	require.NoError(t, err)
	assert.Equal(t, "test-correlation-id-123", mockWriter.writtenID)
	assert.True(t, mockGit.closeCalled)
	assert.True(t, mockFinder.closeCalled)
}

func TestRootCmd_Success_WithDepthFlag(t *testing.T) {
	mockGit := &mockGitRepo{}
	mockFinder := &mockSlipFinder{}
	mockWriter := &mockOutputWriter{}

	deps := &Dependencies{
		LoggerFactory: func() Logger { return &mockLogger{} },
		ConfigLoader: func() (*AppConfig, error) {
			return &AppConfig{Database: "ci"}, nil
		},
		GitRepoFactory: func(_ string, _ Logger) (domain.LocalGitRepository, error) {
			return mockGit, nil
		},
		SlipFinderFactory: func(_ *AppConfig, _ Logger) (domain.SlipFinder, error) {
			return mockFinder, nil
		},
		ResolverFactory: func(_ domain.LocalGitRepository, _ domain.SlipFinder, _ Logger) domain.Resolver {
			return &mockResolver{
				output: &domain.ResolveOutput{
					CorrelationID: "depth-test-id",
				},
			}
		},
		OutputWriterFactory: func() domain.OutputWriter {
			return mockWriter
		},
		Stdout: io.Discard,
		Stderr: io.Discard,
	}

	cmd := NewRootCmdWithDeps(deps)
	cmd.SetArgs([]string{"--depth", "50", "."})

	err := cmd.Execute()

	require.NoError(t, err)
	assert.Equal(t, "depth-test-id", mockWriter.writtenID)
}

func TestRootCmd_Success_WithVerboseFlag(t *testing.T) {
	mockGit := &mockGitRepo{}
	mockFinder := &mockSlipFinder{}
	mockWriter := &mockOutputWriter{}

	deps := &Dependencies{
		LoggerFactory: func() Logger { return &mockLogger{} },
		ConfigLoader: func() (*AppConfig, error) {
			return &AppConfig{Database: "ci"}, nil
		},
		GitRepoFactory: func(_ string, _ Logger) (domain.LocalGitRepository, error) {
			return mockGit, nil
		},
		SlipFinderFactory: func(_ *AppConfig, _ Logger) (domain.SlipFinder, error) {
			return mockFinder, nil
		},
		ResolverFactory: func(_ domain.LocalGitRepository, _ domain.SlipFinder, _ Logger) domain.Resolver {
			return &mockResolver{
				output: &domain.ResolveOutput{
					CorrelationID: "verbose-test-id",
				},
			}
		},
		OutputWriterFactory: func() domain.OutputWriter {
			return mockWriter
		},
		Stdout: io.Discard,
		Stderr: io.Discard,
	}

	cmd := NewRootCmdWithDeps(deps)
	cmd.SetArgs([]string{"-v", "."})

	err := cmd.Execute()

	require.NoError(t, err)
	assert.Equal(t, "verbose-test-id", mockWriter.writtenID)
}

func TestRootCmd_WithCustomPath(t *testing.T) {
	var receivedPath string
	mockGit := &mockGitRepo{}
	mockFinder := &mockSlipFinder{}
	mockWriter := &mockOutputWriter{}

	deps := &Dependencies{
		LoggerFactory: func() Logger { return &mockLogger{} },
		ConfigLoader: func() (*AppConfig, error) {
			return &AppConfig{Database: "ci"}, nil
		},
		GitRepoFactory: func(path string, _ Logger) (domain.LocalGitRepository, error) {
			receivedPath = path
			return mockGit, nil
		},
		SlipFinderFactory: func(_ *AppConfig, _ Logger) (domain.SlipFinder, error) {
			return mockFinder, nil
		},
		ResolverFactory: func(_ domain.LocalGitRepository, _ domain.SlipFinder, _ Logger) domain.Resolver {
			return &mockResolver{
				output: &domain.ResolveOutput{
					CorrelationID: "path-test-id",
				},
			}
		},
		OutputWriterFactory: func() domain.OutputWriter {
			return mockWriter
		},
		Stdout: io.Discard,
		Stderr: io.Discard,
	}

	cmd := NewRootCmdWithDeps(deps)
	cmd.SetArgs([]string{"/custom/repo/path"})

	err := cmd.Execute()

	require.NoError(t, err)
	assert.Equal(t, "/custom/repo/path", receivedPath)
}
