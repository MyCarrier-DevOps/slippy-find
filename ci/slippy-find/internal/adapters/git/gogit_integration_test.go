// Package git provides adapters for interacting with local Git repositories.
package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MyCarrier-DevOps/goLibMyCarrier/logger"

	"github.com/MyCarrier-DevOps/slippy-find/ci/slippy-find/internal/domain"
)

// testLogger is a minimal logger for testing that doesn't output anything.
type testLogger struct{}

func (l *testLogger) Info(_ context.Context, _ string, _ map[string]interface{})           {}
func (l *testLogger) Debug(_ context.Context, _ string, _ map[string]interface{})          {}
func (l *testLogger) Warn(_ context.Context, _ string, _ map[string]interface{})           {}
func (l *testLogger) Warning(_ context.Context, _ string, _ map[string]interface{})        {}
func (l *testLogger) Error(_ context.Context, _ string, _ error, _ map[string]interface{}) {}
func (l *testLogger) WithFields(_ map[string]interface{}) logger.Logger                    { return l }

// setupTestRepo creates a temporary git repository for testing.
// Returns the path to the repository and a cleanup function.
func setupTestRepo(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "slippy-find-test-*")
	require.NoError(t, err)

	cleanup := func() {
		os.RemoveAll(tmpDir)
	}

	// Initialize git repo
	runGit(t, tmpDir, "init")
	runGit(t, tmpDir, "config", "user.email", "test@example.com")
	runGit(t, tmpDir, "config", "user.name", "Test User")

	// Create initial commit
	testFile := filepath.Join(tmpDir, "test.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("initial content"), 0o644))
	runGit(t, tmpDir, "add", ".")
	runGit(t, tmpDir, "commit", "-m", "Initial commit")

	// Add origin remote
	runGit(t, tmpDir, "remote", "add", "origin", "https://github.com/TestOrg/test-repo.git")

	return tmpDir, cleanup
}

// runGit executes a git command in the given directory.
func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\nOutput: %s", args, err, output)
	}
}

func TestNewGoGitRepository_Success(t *testing.T) {
	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	log := &testLogger{}
	repo, err := NewGoGitRepository(repoPath, log)

	require.NoError(t, err)
	require.NotNil(t, repo)
	assert.Equal(t, repoPath, repo.path)

	// Clean up
	require.NoError(t, repo.Close())
}

func TestNewGoGitRepository_NotARepository(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "not-a-repo-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	log := &testLogger{}
	repo, err := NewGoGitRepository(tmpDir, log)

	require.Error(t, err)
	assert.Nil(t, repo)
	assert.ErrorIs(t, err, domain.ErrRepositoryNotFound)
}

func TestGoGitRepository_GetGitContext_Success(t *testing.T) {
	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	log := &testLogger{}
	repo, err := NewGoGitRepository(repoPath, log)
	require.NoError(t, err)
	defer repo.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	gitCtx, err := repo.GetGitContext(ctx)

	require.NoError(t, err)
	require.NotNil(t, gitCtx)
	assert.NotEmpty(t, gitCtx.HeadSHA)
	assert.Len(t, gitCtx.HeadSHA, 40) // Full SHA length
	// Default branch is "main" in modern Git, "master" in older versions
	assert.True(t, gitCtx.Branch == "main" || gitCtx.Branch == "master")
	assert.Equal(t, "TestOrg/test-repo", gitCtx.Repository)
	assert.False(t, gitCtx.IsDetached)
}

func TestGoGitRepository_GetGitContext_NoOriginRemote(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "no-origin-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create repo without origin remote
	runGit(t, tmpDir, "init")
	runGit(t, tmpDir, "config", "user.email", "test@example.com")
	runGit(t, tmpDir, "config", "user.name", "Test User")
	testFile := filepath.Join(tmpDir, "test.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("content"), 0o644))
	runGit(t, tmpDir, "add", ".")
	runGit(t, tmpDir, "commit", "-m", "Initial commit")

	log := &testLogger{}
	repo, err := NewGoGitRepository(tmpDir, log)
	require.NoError(t, err)
	defer repo.Close()

	ctx := context.Background()
	gitCtx, err := repo.GetGitContext(ctx)

	require.Error(t, err)
	assert.Nil(t, gitCtx)
	assert.ErrorIs(t, err, domain.ErrNoRemoteOrigin)
}

func TestGoGitRepository_GetGitContext_DetachedHead(t *testing.T) {
	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	// Create another commit to have history
	testFile := filepath.Join(repoPath, "test.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("modified content"), 0o644))
	runGit(t, repoPath, "add", ".")
	runGit(t, repoPath, "commit", "-m", "Second commit")

	// Get first commit SHA and checkout detached
	cmd := exec.Command("git", "rev-parse", "HEAD~1")
	cmd.Dir = repoPath
	output, err := cmd.Output()
	require.NoError(t, err)
	firstCommit := string(output[:len(output)-1]) // Remove newline

	runGit(t, repoPath, "checkout", firstCommit)

	log := &testLogger{}
	repo, err := NewGoGitRepository(repoPath, log)
	require.NoError(t, err)
	defer repo.Close()

	ctx := context.Background()
	gitCtx, err := repo.GetGitContext(ctx)

	require.NoError(t, err)
	require.NotNil(t, gitCtx)
	assert.True(t, gitCtx.IsDetached)
	assert.Empty(t, gitCtx.Branch)
	assert.Equal(t, firstCommit, gitCtx.HeadSHA)
}

func TestGoGitRepository_GetCommitAncestry_Success(t *testing.T) {
	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	// Create a few more commits
	for i := 0; i < 5; i++ {
		testFile := filepath.Join(repoPath, "test.txt")
		require.NoError(t, os.WriteFile(testFile, []byte("content "+string(rune('a'+i))), 0o644))
		runGit(t, repoPath, "add", ".")
		runGit(t, repoPath, "commit", "-m", "Commit "+string(rune('A'+i)))
	}

	log := &testLogger{}
	repo, err := NewGoGitRepository(repoPath, log)
	require.NoError(t, err)
	defer repo.Close()

	ctx := context.Background()
	commits, err := repo.GetCommitAncestry(ctx, 10)

	require.NoError(t, err)
	// 1 initial commit + 5 additional = 6 total
	assert.Len(t, commits, 6)

	// First commit should be HEAD
	gitCtx, err := repo.GetGitContext(ctx)
	require.NoError(t, err)
	assert.Equal(t, gitCtx.HeadSHA, commits[0])
}

func TestGoGitRepository_GetCommitAncestry_DepthLimit(t *testing.T) {
	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	// Create 10 more commits
	for i := 0; i < 10; i++ {
		testFile := filepath.Join(repoPath, "test.txt")
		require.NoError(t, os.WriteFile(testFile, []byte("content "+string(rune('a'+i))), 0o644))
		runGit(t, repoPath, "add", ".")
		runGit(t, repoPath, "commit", "-m", "Commit "+string(rune('A'+i)))
	}

	log := &testLogger{}
	repo, err := NewGoGitRepository(repoPath, log)
	require.NoError(t, err)
	defer repo.Close()

	ctx := context.Background()
	commits, err := repo.GetCommitAncestry(ctx, 5)

	require.NoError(t, err)
	// Should be limited to 5
	assert.Len(t, commits, 5)
}

func TestGoGitRepository_GetCommitAncestry_ZeroDepth(t *testing.T) {
	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	log := &testLogger{}
	repo, err := NewGoGitRepository(repoPath, log)
	require.NoError(t, err)
	defer repo.Close()

	ctx := context.Background()
	commits, err := repo.GetCommitAncestry(ctx, 0)

	require.NoError(t, err)
	// Should use default depth (25) but repo only has 1 commit
	assert.Len(t, commits, 1)
}

func TestGoGitRepository_GetCommitAncestry_ContextCancellation(t *testing.T) {
	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	log := &testLogger{}
	repo, err := NewGoGitRepository(repoPath, log)
	require.NoError(t, err)
	defer repo.Close()

	// Create canceled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	commits, err := repo.GetCommitAncestry(ctx, 10)

	require.Error(t, err)
	assert.Nil(t, commits)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestGoGitRepository_Close(t *testing.T) {
	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	log := &testLogger{}
	repo, err := NewGoGitRepository(repoPath, log)
	require.NoError(t, err)

	err = repo.Close()
	require.NoError(t, err)
}
