package hive

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMatchRemotePattern(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		pattern string
		remote  string
		want    bool
		wantErr bool
	}{
		{
			name:    "empty pattern matches all",
			pattern: "",
			remote:  "https://github.com/test/repo",
			want:    true,
		},
		{
			name:    "exact match",
			pattern: "https://github.com/test/repo",
			remote:  "https://github.com/test/repo",
			want:    true,
		},
		{
			name:    "regex match",
			pattern: ".*/test/repo",
			remote:  "https://github.com/test/repo",
			want:    true,
		},
		{
			name:    "regex no match",
			pattern: ".*/other/repo",
			remote:  "https://github.com/test/repo",
			want:    false,
		},
		{
			name:    "invalid regex",
			pattern: "[invalid",
			remote:  "https://github.com/test/repo",
			wantErr: true,
		},
		{
			name:    "partial match",
			pattern: "github.com",
			remote:  "https://github.com/test/repo",
			want:    true,
		},
		{
			name:    "anchor pattern",
			pattern: "^https://github.com/.*",
			remote:  "https://github.com/test/repo",
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := matchRemotePattern(tt.pattern, tt.remote)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
