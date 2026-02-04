// Package output provides adapters for writing application output.
package output

import (
	"fmt"
	"io"
	"os"
)

// Writer writes the correlation ID to the configured output destination.
// By default, it writes to stdout.
type Writer struct {
	out io.Writer
}

// NewWriter creates a new Writer that writes to stdout.
func NewWriter() *Writer {
	return &Writer{out: os.Stdout}
}

// NewWriterWithOutput creates a new Writer with a custom output destination.
// This is useful for testing.
func NewWriterWithOutput(out io.Writer) *Writer {
	return &Writer{out: out}
}

// WriteCorrelationID writes the correlation ID to the output destination.
// The correlation ID is written as a single line without any prefix or formatting.
func (w *Writer) WriteCorrelationID(correlationID string) error {
	_, err := fmt.Fprintln(w.out, correlationID)
	return err
}
