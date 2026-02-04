package usecases

import (
	"context"
	"errors"
	"testing"

	"github.com/MyCarrier-DevOps/slippy-find/ci/slippy-find/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockLogger implements the Logger interface for testing.
type mockLogger struct{}

func (m *mockLogger) Info(_ context.Context, _ string, _ map[string]interface{})           {}
func (m *mockLogger) Debug(_ context.Context, _ string, _ map[string]interface{})          {}
func (m *mockLogger) Warn(_ context.Context, _ string, _ map[string]interface{})           {}
func (m *mockLogger) Error(_ context.Context, _ string, _ error, _ map[string]interface{}) {}

// mockLocalGitRepository implements domain.LocalGitRepository for testing.
type mockLocalGitRepository struct {
	gitContext    *domain.GitContext
	gitContextErr error
	commits       []string
	commitsErr    error
	closeCalled   bool
}

func (m *mockLocalGitRepository) GetGitContext(_ context.Context) (*domain.GitContext, error) {
	if m.gitContextErr != nil {
		return nil, m.gitContextErr
	}
	return m.gitContext, nil
}

func (m *mockLocalGitRepository) GetCommitAncestry(_ context.Context, _ int) ([]string, error) {
	if m.commitsErr != nil {
		return nil, m.commitsErr
	}
	return m.commits, nil
}

func (m *mockLocalGitRepository) Close() error {
	m.closeCalled = true
	return nil
}

// mockSlipFinder implements domain.SlipFinder for testing.
type mockSlipFinder struct {
	findByCommitsSlip   *domain.Slip
	findByCommitsCommit string
	findByCommitsErr    error
	findByCommitsCalls  []findByCommitsCall
	closeCalled         bool
}

type findByCommitsCall struct {
	repository string
	commits    []string
}

func (m *mockSlipFinder) FindByCommits(_ context.Context, repository string, commits []string) (*domain.Slip, string, error) {
	m.findByCommitsCalls = append(m.findByCommitsCalls, findByCommitsCall{
		repository: repository,
		commits:    commits,
	})
	return m.findByCommitsSlip, m.findByCommitsCommit, m.findByCommitsErr
}

func (m *mockSlipFinder) Close() error {
	m.closeCalled = true
	return nil
}

func TestSlipResolver_Resolve(t *testing.T) {
	tests := []struct {
		name       string
		input      domain.ResolveInput
		mockGit    *mockLocalGitRepository
		mockFinder *mockSlipFinder
		wantOutput *domain.ResolveOutput
		wantErr    bool
		wantErrMsg string
	}{
		{
			name: "successful resolution - slip found in ancestry",
			input: domain.ResolveInput{
				Depth: 10,
			},
			mockGit: &mockLocalGitRepository{
				gitContext: &domain.GitContext{
					HeadSHA:    "abc123def456",
					Branch:     "feature/test",
					Repository: "MyCarrier-DevOps/test-repo",
					IsDetached: false,
				},
				commits: []string{"abc123def456", "def456ghi789", "ghi789jkl012"},
			},
			mockFinder: &mockSlipFinder{
				findByCommitsSlip: &domain.Slip{
					CorrelationID: "test-correlation-id-123",
				},
				findByCommitsCommit: "def456ghi789",
			},
			wantOutput: &domain.ResolveOutput{
				CorrelationID: "test-correlation-id-123",
				MatchedCommit: "def456ghi789",
				Repository:    "MyCarrier-DevOps/test-repo",
				Branch:        "feature/test",
				ResolvedBy:    "ancestry",
			},
			wantErr: false,
		},
		{
			name: "successful resolution - HEAD commit matches",
			input: domain.ResolveInput{
				Depth: 25,
			},
			mockGit: &mockLocalGitRepository{
				gitContext: &domain.GitContext{
					HeadSHA:    "head123456",
					Branch:     "main",
					Repository: "MyCarrier-DevOps/another-repo",
					IsDetached: false,
				},
				commits: []string{"head123456"},
			},
			mockFinder: &mockSlipFinder{
				findByCommitsSlip: &domain.Slip{
					CorrelationID: "head-match-correlation",
				},
				findByCommitsCommit: "head123456",
			},
			wantOutput: &domain.ResolveOutput{
				CorrelationID: "head-match-correlation",
				MatchedCommit: "head123456",
				Repository:    "MyCarrier-DevOps/another-repo",
				Branch:        "main",
				ResolvedBy:    "ancestry",
			},
			wantErr: false,
		},
		{
			name: "successful resolution - detached HEAD",
			input: domain.ResolveInput{
				Depth: 10,
			},
			mockGit: &mockLocalGitRepository{
				gitContext: &domain.GitContext{
					HeadSHA:    "detached123",
					Branch:     "",
					Repository: "MyCarrier-DevOps/detached-repo",
					IsDetached: true,
				},
				commits: []string{"detached123", "parent456"},
			},
			mockFinder: &mockSlipFinder{
				findByCommitsSlip: &domain.Slip{
					CorrelationID: "detached-correlation",
				},
				findByCommitsCommit: "detached123",
			},
			wantOutput: &domain.ResolveOutput{
				CorrelationID: "detached-correlation",
				MatchedCommit: "detached123",
				Repository:    "MyCarrier-DevOps/detached-repo",
				Branch:        "",
				ResolvedBy:    "ancestry",
			},
			wantErr: false,
		},
		{
			name: "uses default depth when not specified",
			input: domain.ResolveInput{
				Depth: 0, // Should use default
			},
			mockGit: &mockLocalGitRepository{
				gitContext: &domain.GitContext{
					HeadSHA:    "abc123",
					Branch:     "main",
					Repository: "MyCarrier-DevOps/test",
					IsDetached: false,
				},
				commits: []string{"abc123"},
			},
			mockFinder: &mockSlipFinder{
				findByCommitsSlip: &domain.Slip{
					CorrelationID: "default-depth-correlation",
				},
				findByCommitsCommit: "abc123",
			},
			wantOutput: &domain.ResolveOutput{
				CorrelationID: "default-depth-correlation",
				MatchedCommit: "abc123",
				Repository:    "MyCarrier-DevOps/test",
				Branch:        "main",
				ResolvedBy:    "ancestry",
			},
			wantErr: false,
		},
		{
			name: "error - git context fails",
			input: domain.ResolveInput{
				Depth: 10,
			},
			mockGit: &mockLocalGitRepository{
				gitContextErr: errors.New("failed to get HEAD"),
			},
			mockFinder: &mockSlipFinder{},
			wantErr:    true,
			wantErrMsg: "failed to get git context",
		},
		{
			name: "error - no origin remote",
			input: domain.ResolveInput{
				Depth: 10,
			},
			mockGit: &mockLocalGitRepository{
				gitContextErr: domain.ErrNoRemoteOrigin,
			},
			mockFinder: &mockSlipFinder{},
			wantErr:    true,
			wantErrMsg: "no 'origin' remote configured",
		},
		{
			name: "error - commit ancestry fails",
			input: domain.ResolveInput{
				Depth: 10,
			},
			mockGit: &mockLocalGitRepository{
				gitContext: &domain.GitContext{
					HeadSHA:    "abc123",
					Branch:     "main",
					Repository: "MyCarrier-DevOps/test",
					IsDetached: false,
				},
				commitsErr: errors.New("failed to walk commits"),
			},
			mockFinder: &mockSlipFinder{},
			wantErr:    true,
			wantErrMsg: "failed to get commit ancestry",
		},
		{
			name: "error - store query fails",
			input: domain.ResolveInput{
				Depth: 10,
			},
			mockGit: &mockLocalGitRepository{
				gitContext: &domain.GitContext{
					HeadSHA:    "abc123",
					Branch:     "main",
					Repository: "MyCarrier-DevOps/test",
					IsDetached: false,
				},
				commits: []string{"abc123"},
			},
			mockFinder: &mockSlipFinder{
				findByCommitsErr: errors.New("database connection failed"),
			},
			wantErr:    true,
			wantErrMsg: "failed to find slip by commits",
		},
		{
			name: "error - no slip found in ancestry",
			input: domain.ResolveInput{
				Depth: 10,
			},
			mockGit: &mockLocalGitRepository{
				gitContext: &domain.GitContext{
					HeadSHA:    "abc123",
					Branch:     "main",
					Repository: "MyCarrier-DevOps/test",
					IsDetached: false,
				},
				commits: []string{"abc123", "def456"},
			},
			mockFinder: &mockSlipFinder{
				findByCommitsSlip:   nil, // No slip found
				findByCommitsCommit: "",
			},
			wantErr:    true,
			wantErrMsg: "no slip found in commit ancestry",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			resolver := NewSlipResolver(tt.mockGit, tt.mockFinder, &mockLogger{})

			// Act
			output, err := resolver.Resolve(context.Background(), tt.input)

			// Assert
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErrMsg)
				assert.Nil(t, output)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, output)
			assert.Equal(t, tt.wantOutput.CorrelationID, output.CorrelationID)
			assert.Equal(t, tt.wantOutput.MatchedCommit, output.MatchedCommit)
			assert.Equal(t, tt.wantOutput.Repository, output.Repository)
			assert.Equal(t, tt.wantOutput.Branch, output.Branch)
			assert.Equal(t, tt.wantOutput.ResolvedBy, output.ResolvedBy)
		})
	}
}

func TestSlipResolver_Resolve_StoreCalledWithCorrectArgs(t *testing.T) {
	// Arrange
	mockGit := &mockLocalGitRepository{
		gitContext: &domain.GitContext{
			HeadSHA:    "abc123",
			Branch:     "main",
			Repository: "MyCarrier-DevOps/test-repo",
			IsDetached: false,
		},
		commits: []string{"abc123", "def456", "ghi789"},
	}
	mockFinder := &mockSlipFinder{
		findByCommitsSlip: &domain.Slip{
			CorrelationID: "test-correlation",
		},
		findByCommitsCommit: "abc123",
	}
	resolver := NewSlipResolver(mockGit, mockFinder, &mockLogger{})

	// Act
	_, err := resolver.Resolve(context.Background(), domain.ResolveInput{Depth: 10})

	// Assert
	require.NoError(t, err)
	require.Len(t, mockFinder.findByCommitsCalls, 1)
	call := mockFinder.findByCommitsCalls[0]
	assert.Equal(t, "MyCarrier-DevOps/test-repo", call.repository)
	assert.Equal(t, []string{"abc123", "def456", "ghi789"}, call.commits)
}
