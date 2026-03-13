package store

import (
	"context"
	"testing"
	"time"

	"github.com/MyCarrier-DevOps/goLibMyCarrier/slippy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Compile-time check: mockSlipStore must satisfy slippy.SlipStore.
// If the interface gains or changes methods, this line fails to compile.
var _ slippy.SlipStore = (*mockSlipStore)(nil)

func TestSlipStoreInterface_InsertAncestryLink(t *testing.T) {
	store := &mockSlipStore{}
	err := store.InsertAncestryLink(context.Background(), &slippy.Slip{
		CorrelationID: "child-1",
		Repository:    "org/repo",
		Branch:        "main",
		CommitSHA:     "aaa111",
	}, slippy.AncestryEntry{
		CorrelationID: "parent-1",
		CommitSHA:     "bbb222",
		Status:        slippy.SlipStatusFailed,
		FailedStep:    "build",
		CreatedAt:     time.Now(),
	})
	require.NoError(t, err)
}

func TestSlipStoreInterface_ResolveAncestry(t *testing.T) {
	store := &mockSlipStore{}
	entries, err := store.ResolveAncestry(
		context.Background(),
		"org/repo", "main", "child-1", 10,
	)
	require.NoError(t, err)
	assert.Nil(t, entries)
}

// TestAncestryEntryFields verifies the AncestryEntry struct has the expected fields.
func TestAncestryEntryFields(t *testing.T) {
	entry := slippy.AncestryEntry{
		CorrelationID: "abc",
		CommitSHA:     "def",
		Status:        slippy.SlipStatusCompleted,
		FailedStep:    "",
		CreatedAt:     time.Now(),
	}
	assert.Equal(t, "abc", entry.CorrelationID)
	assert.Equal(t, "def", entry.CommitSHA)
	assert.Equal(t, slippy.SlipStatusCompleted, entry.Status)
}
