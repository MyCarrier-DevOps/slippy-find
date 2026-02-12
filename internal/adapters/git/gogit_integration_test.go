// Package git provides adapters for interacting with local Git repositories.
package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MyCarrier-DevOps/goLibMyCarrier/logger"

	"github.com/MyCarrier-DevOps/slippy-find/internal/domain"
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

// TestGoGitRepository_GetCommitAncestry_FirstParentOnly tests that merge commits
// from other branches are excluded from the ancestry chain. This prevents incorrect
// slip resolution when the default branch is merged into a feature branch.
func TestGoGitRepository_GetCommitAncestry_FirstParentOnly(t *testing.T) {
	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	// Capture the default branch name before switching branches
	defaultBranch := getGitOutput(t, repoPath, "branch", "--show-current")

	// Create a feature-branch commit
	testFile := filepath.Join(repoPath, "feature.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("feature work"), 0o644))
	runGit(t, repoPath, "add", ".")
	runGit(t, repoPath, "commit", "-m", "Feature commit 1")

	// Record the feature commit SHA
	featureCommit1 := getGitOutput(t, repoPath, "rev-parse", "HEAD")

	// Create a side branch simulating main with its own commits
	runGit(t, repoPath, "checkout", "-b", "simulated-main", "HEAD~1")
	mainFile := filepath.Join(repoPath, "main-change.txt")
	require.NoError(t, os.WriteFile(mainFile, []byte("main work 1"), 0o644))
	runGit(t, repoPath, "add", ".")
	runGit(t, repoPath, "commit", "-m", "Main commit 1")
	mainCommit1 := getGitOutput(t, repoPath, "rev-parse", "HEAD")

	require.NoError(t, os.WriteFile(mainFile, []byte("main work 2"), 0o644))
	runGit(t, repoPath, "add", ".")
	runGit(t, repoPath, "commit", "-m", "Main commit 2")
	mainCommit2 := getGitOutput(t, repoPath, "rev-parse", "HEAD")

	// Switch back to the feature branch
	runGit(t, repoPath, "checkout", defaultBranch)

	// Merge simulated-main into the feature branch (creates a merge commit)
	runGit(t, repoPath, "merge", "simulated-main", "-m", "Merge main into feature")
	mergeCommit := getGitOutput(t, repoPath, "rev-parse", "HEAD")

	// Create one more feature commit after the merge
	require.NoError(t, os.WriteFile(testFile, []byte("feature work 2"), 0o644))
	runGit(t, repoPath, "add", ".")
	runGit(t, repoPath, "commit", "-m", "Feature commit 2")
	featureCommit2 := getGitOutput(t, repoPath, "rev-parse", "HEAD")

	// Now get ancestry — should follow first-parent only
	log := &testLogger{}
	repo, err := NewGoGitRepository(repoPath, log)
	require.NoError(t, err)
	defer repo.Close()

	ctx := context.Background()
	commits, err := repo.GetCommitAncestry(ctx, 20)
	require.NoError(t, err)

	// First-parent chain: featureCommit2 -> mergeCommit -> featureCommit1 -> initial
	// The main branch commits should NOT appear
	assert.Contains(t, commits, featureCommit2, "latest feature commit should be in ancestry")
	assert.Contains(t, commits, mergeCommit, "merge commit should be in ancestry")
	assert.Contains(t, commits, featureCommit1, "feature commit 1 should be in ancestry")
	assert.NotContains(t, commits, mainCommit1, "main branch commit 1 should be excluded")
	assert.NotContains(t, commits, mainCommit2, "main branch commit 2 should be excluded")

	// Verify ordering: featureCommit2 comes first (HEAD)
	assert.Equal(t, featureCommit2, commits[0], "HEAD should be the first commit")
}

// getGitOutput runs a git command and returns its trimmed stdout.
func getGitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.Output()
	require.NoError(t, err, "git %v failed", args)
	return strings.TrimSpace(string(output))
}
