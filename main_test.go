package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewConfigTypeError(t *testing.T) {
	err := newConfigTypeError("*expected.Type")

	assert.NotNil(t, err)
	assert.IsType(t, &configTypeError{}, err)
}

func TestConfigTypeError_Error(t *testing.T) {
	tests := []struct {
		name     string
		expected string
		want     string
	}{
		{
			name:     "ClickHouse config type",
			expected: "*ch.ClickhouseConfig",
			want:     "invalid configuration type: expected *ch.ClickhouseConfig",
		},
		{
			name:     "Pipeline config type",
			expected: "*slippy.PipelineConfig",
			want:     "invalid configuration type: expected *slippy.PipelineConfig",
		},
		{
			name:     "empty expected type",
			expected: "",
			want:     "invalid configuration type: expected ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &configTypeError{expected: tt.expected}
			assert.Equal(t, tt.want, err.Error())
		})
	}
}

func TestConfigTypeError_ImplementsError(t *testing.T) {
	var err error = newConfigTypeError("test")
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "test")
}
