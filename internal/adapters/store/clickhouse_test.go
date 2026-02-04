// Package store provides adapters for slip storage backends.
package store

import (
	"context"
	"errors"
	"testing"

	"github.com/MyCarrier-DevOps/goLibMyCarrier/slippy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockSlipStore implements slippy.SlipStore for testing.
type mockSlipStore struct {
	findByCommitsSlip   *slippy.Slip
	findByCommitsCommit string
	findByCommitsErr    error
	closeErr            error
	closeCalled         bool
}

func (m *mockSlipStore) FindByCommits(
	_ context.Context,
	_ string,
	_ []string,
) (*slippy.Slip, string, error) {
	return m.findByCommitsSlip, m.findByCommitsCommit, m.findByCommitsErr
}

func (m *mockSlipStore) Close() error {
	m.closeCalled = true
	return m.closeErr
}

// Implement other SlipStore methods as no-ops to satisfy the interface.
func (m *mockSlipStore) Create(_ context.Context, _ *slippy.Slip) error { return nil }
func (m *mockSlipStore) Load(_ context.Context, _ string) (*slippy.Slip, error) {
	return nil, nil
}
func (m *mockSlipStore) LoadByCommit(_ context.Context, _, _ string) (*slippy.Slip, error) {
	return nil, nil
}
func (m *mockSlipStore) Update(_ context.Context, _ *slippy.Slip) error { return nil }
func (m *mockSlipStore) UpdateStep(
	_ context.Context,
	_, _, _ string,
	_ slippy.StepStatus,
) error {
	return nil
}
func (m *mockSlipStore) UpdateStepWithHistory(
	_ context.Context,
	_, _, _ string,
	_ slippy.StepStatus,
	_ slippy.StateHistoryEntry,
) error {
	return nil
}
func (m *mockSlipStore) UpdateComponentStatus(
	_ context.Context,
	_, _, _ string,
	_ slippy.StepStatus,
) error {
	return nil
}
func (m *mockSlipStore) AppendHistory(
	_ context.Context,
	_ string,
	_ slippy.StateHistoryEntry,
) error {
	return nil
}
func (m *mockSlipStore) FindAllByCommits(
	_ context.Context,
	_ string,
	_ []string,
) ([]slippy.SlipWithCommit, error) {
	return nil, nil
}

func TestNewClickHouseAdapter(t *testing.T) {
	mockStore := &mockSlipStore{}
	adapter := NewClickHouseAdapter(mockStore)

	require.NotNil(t, adapter)
	assert.Equal(t, mockStore, adapter.store)
}

func TestClickHouseAdapter_FindByCommits_Success(t *testing.T) {
	mockStore := &mockSlipStore{
		findByCommitsSlip: &slippy.Slip{
			CorrelationID: "test-correlation-id",
		},
		findByCommitsCommit: "abc123",
	}
	adapter := NewClickHouseAdapter(mockStore)

	slip, matchedCommit, err := adapter.FindByCommits(
		context.Background(),
		"test/repo",
		[]string{"abc123", "def456"},
	)

	require.NoError(t, err)
	require.NotNil(t, slip)
	assert.Equal(t, "test-correlation-id", slip.CorrelationID)
	assert.Equal(t, "abc123", matchedCommit)
}

func TestClickHouseAdapter_FindByCommits_NotFound(t *testing.T) {
	mockStore := &mockSlipStore{
		findByCommitsSlip:   nil,
		findByCommitsCommit: "",
	}
	adapter := NewClickHouseAdapter(mockStore)

	slip, matchedCommit, err := adapter.FindByCommits(
		context.Background(),
		"test/repo",
		[]string{"abc123"},
	)

	require.NoError(t, err)
	assert.Nil(t, slip)
	assert.Equal(t, "", matchedCommit)
}

func TestClickHouseAdapter_FindByCommits_Error(t *testing.T) {
	mockStore := &mockSlipStore{
		findByCommitsErr: errors.New("database connection failed"),
	}
	adapter := NewClickHouseAdapter(mockStore)

	slip, matchedCommit, err := adapter.FindByCommits(
		context.Background(),
		"test/repo",
		[]string{"abc123"},
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "database connection failed")
	assert.Nil(t, slip)
	assert.Equal(t, "", matchedCommit)
}

func TestClickHouseAdapter_Close_Success(t *testing.T) {
	mockStore := &mockSlipStore{}
	adapter := NewClickHouseAdapter(mockStore)

	err := adapter.Close()

	require.NoError(t, err)
	assert.True(t, mockStore.closeCalled)
}

func TestClickHouseAdapter_Close_Error(t *testing.T) {
	mockStore := &mockSlipStore{
		closeErr: errors.New("close failed"),
	}
	adapter := NewClickHouseAdapter(mockStore)

	err := adapter.Close()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "close failed")
	assert.True(t, mockStore.closeCalled)
}
