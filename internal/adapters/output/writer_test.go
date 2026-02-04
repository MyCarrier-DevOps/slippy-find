package output

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriter_WriteCorrelationID(t *testing.T) {
	tests := []struct {
		name          string
		correlationID string
		wantOutput    string
	}{
		{
			name:          "simple correlation ID",
			correlationID: "abc123",
			wantOutput:    "abc123\n",
		},
		{
			name:          "UUID-style correlation ID",
			correlationID: "550e8400-e29b-41d4-a716-446655440000",
			wantOutput:    "550e8400-e29b-41d4-a716-446655440000\n",
		},
		{
			name:          "empty correlation ID",
			correlationID: "",
			wantOutput:    "\n",
		},
		{
			name:          "correlation ID with special characters",
			correlationID: "slip-abc_123.test",
			wantOutput:    "slip-abc_123.test\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			var buf bytes.Buffer
			writer := NewWriterWithOutput(&buf)

			// Act
			err := writer.WriteCorrelationID(tt.correlationID)

			// Assert
			require.NoError(t, err)
			assert.Equal(t, tt.wantOutput, buf.String())
		})
	}
}

func TestNewWriter_UsesStdout(t *testing.T) {
	writer := NewWriter()
	assert.NotNil(t, writer)
	assert.NotNil(t, writer.out)
}
