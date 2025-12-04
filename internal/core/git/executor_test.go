package git

import (
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseDiffStats(t *testing.T) {
	tests := []struct {
		name    string
		output  string
		wantAdd int
		wantDel int
		wantErr bool
	}{
		{
			name:    "empty output means clean",
			output:  "",
			wantAdd: 0,
			wantDel: 0,
		},
		{
			name:    "whitespace only means clean",
			output:  "   \n  ",
			wantAdd: 0,
			wantDel: 0,
		},
		{
			name:    "insertions and deletions",
			output:  " 3 files changed, 10 insertions(+), 5 deletions(-)",
			wantAdd: 10,
			wantDel: 5,
		},
		{
			name:    "insertions only",
			output:  " 1 file changed, 25 insertions(+)",
			wantAdd: 25,
			wantDel: 0,
		},
		{
			name:    "deletions only",
			output:  " 2 files changed, 15 deletions(-)",
			wantAdd: 0,
			wantDel: 15,
		},
		{
			name:    "single insertion",
			output:  " 1 file changed, 1 insertion(+)",
			wantAdd: 1,
			wantDel: 0,
		},
		{
			name:    "single deletion",
			output:  " 1 file changed, 1 deletion(-)",
			wantAdd: 0,
			wantDel: 1,
		},
		{
			name:    "large numbers",
			output:  " 50 files changed, 1234 insertions(+), 567 deletions(-)",
			wantAdd: 1234,
			wantDel: 567,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			add, del, err := parseDiffStats(tt.output)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantAdd, add, "additions mismatch")
			assert.Equal(t, tt.wantDel, del, "deletions mismatch")
		})
	}
}

// mockExecutor is a simple mock for testing git executor methods.
type mockExecutor struct {
	runDirFunc func(ctx context.Context, dir, cmd string, args ...string) ([]byte, error)
}

func (m *mockExecutor) Run(ctx context.Context, cmd string, args ...string) ([]byte, error) {
	return nil, nil
}

func (m *mockExecutor) RunDir(ctx context.Context, dir, cmd string, args ...string) ([]byte, error) {
	if m.runDirFunc != nil {
		return m.runDirFunc(ctx, dir, cmd, args...)
	}
	return nil, nil
}

func (m *mockExecutor) RunStream(ctx context.Context, stdout, stderr io.Writer, cmd string, args ...string) error {
	return nil
}

func (m *mockExecutor) RunDirStream(ctx context.Context, dir string, stdout, stderr io.Writer, cmd string, args ...string) error {
	return nil
}

func TestExecutor_Branch(t *testing.T) {
	tests := []struct {
		name        string
		branchOut   string
		revParseOut string
		want        string
		wantErr     bool
	}{
		{
			name:      "normal branch",
			branchOut: "main\n",
			want:      "main",
		},
		{
			name:      "feature branch with slashes",
			branchOut: "feature/my-feature\n",
			want:      "feature/my-feature",
		},
		{
			name:        "detached HEAD",
			branchOut:   "\n",
			revParseOut: "abc1234\n",
			want:        "abc1234",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			callCount := 0
			mock := &mockExecutor{
				runDirFunc: func(ctx context.Context, dir, cmd string, args ...string) ([]byte, error) {
					callCount++
					if callCount == 1 {
						// First call is branch --show-current
						return []byte(tt.branchOut), nil
					}
					// Second call is rev-parse --short HEAD
					return []byte(tt.revParseOut), nil
				},
			}

			e := NewExecutor("git", mock)
			got, err := e.Branch(context.Background(), "/test/dir")

			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestExecutor_DiffStats(t *testing.T) {
	tests := []struct {
		name    string
		output  string
		wantAdd int
		wantDel int
		wantErr bool
	}{
		{
			name:    "with changes",
			output:  " 2 files changed, 10 insertions(+), 5 deletions(-)\n",
			wantAdd: 10,
			wantDel: 5,
		},
		{
			name:    "no changes",
			output:  "",
			wantAdd: 0,
			wantDel: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockExecutor{
				runDirFunc: func(ctx context.Context, dir, cmd string, args ...string) ([]byte, error) {
					return []byte(tt.output), nil
				},
			}

			e := NewExecutor("git", mock)
			add, del, err := e.DiffStats(context.Background(), "/test/dir")

			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantAdd, add)
			assert.Equal(t, tt.wantDel, del)
		})
	}
}
