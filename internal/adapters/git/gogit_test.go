package git

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseRepoFromURL(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		wantRepo string
		wantErr  bool
	}{
		{
			name:     "HTTPS URL with .git suffix",
			url:      "https://github.com/MyCarrier-DevOps/slippy-find.git",
			wantRepo: "MyCarrier-DevOps/slippy-find",
			wantErr:  false,
		},
		{
			name:     "HTTPS URL without .git suffix",
			url:      "https://github.com/MyCarrier-DevOps/slippy-find",
			wantRepo: "MyCarrier-DevOps/slippy-find",
			wantErr:  false,
		},
		{
			name:     "SSH URL with .git suffix",
			url:      "git@github.com:MyCarrier-DevOps/slippy-find.git",
			wantRepo: "MyCarrier-DevOps/slippy-find",
			wantErr:  false,
		},
		{
			name:     "SSH URL without .git suffix",
			url:      "git@github.com:MyCarrier-DevOps/slippy-find",
			wantRepo: "MyCarrier-DevOps/slippy-find",
			wantErr:  false,
		},
		{
			name:     "HTTPS URL with different host",
			url:      "https://gitlab.com/owner/project.git",
			wantRepo: "owner/project",
			wantErr:  false,
		},
		{
			name:     "SSH URL with different host",
			url:      "git@gitlab.com:owner/project.git",
			wantRepo: "owner/project",
			wantErr:  false,
		},
		{
			name:     "URL with whitespace trimmed",
			url:      "  https://github.com/owner/repo.git  ",
			wantRepo: "owner/repo",
			wantErr:  false,
		},
		{
			name:     "HTTP URL (not HTTPS)",
			url:      "http://github.com/owner/repo.git",
			wantRepo: "owner/repo",
			wantErr:  false,
		},
		{
			name:    "invalid URL - no path",
			url:     "https://github.com",
			wantErr: true,
		},
		{
			name:    "invalid URL - only owner",
			url:     "https://github.com/owner",
			wantErr: true,
		},
		{
			name:    "invalid URL - empty string",
			url:     "",
			wantErr: true,
		},
		{
			name:    "invalid URL - random string",
			url:     "not-a-url",
			wantErr: true,
		},
		{
			name:    "invalid URL - file path",
			url:     "/path/to/repo",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, err := parseRepoFromURL(tt.url)

			if tt.wantErr {
				require.Error(t, err, "expected error for URL: %s", tt.url)
				return
			}

			require.NoError(t, err, "unexpected error for URL: %s", tt.url)
			assert.Equal(t, tt.wantRepo, repo, "repository name mismatch")
		})
	}
}
