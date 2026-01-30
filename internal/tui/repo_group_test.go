package tui

import (
	"testing"

	"github.com/hay-kot/hive/internal/core/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGroupSessionsByRepo(t *testing.T) {
	tests := []struct {
		name        string
		sessions    []session.Session
		localRemote string
		wantGroups  []struct {
			name     string
			remote   string
			sessions []string // session names in expected order
		}
	}{
		{
			name:     "empty sessions returns nil",
			sessions: nil,
			wantGroups: []struct {
				name     string
				remote   string
				sessions []string
			}{},
		},
		{
			name: "single repo groups all sessions",
			sessions: []session.Session{
				{Name: "session-b", Remote: "git@github.com:user/hive.git"},
				{Name: "session-a", Remote: "git@github.com:user/hive.git"},
			},
			wantGroups: []struct {
				name     string
				remote   string
				sessions []string
			}{
				{name: "hive", remote: "git@github.com:user/hive.git", sessions: []string{"session-a", "session-b"}},
			},
		},
		{
			name: "multiple repos sorted alphabetically",
			sessions: []session.Session{
				{Name: "s1", Remote: "git@github.com:user/zebra.git"},
				{Name: "s2", Remote: "git@github.com:user/alpha.git"},
				{Name: "s3", Remote: "git@github.com:user/beta.git"},
			},
			wantGroups: []struct {
				name     string
				remote   string
				sessions []string
			}{
				{name: "alpha", remote: "git@github.com:user/alpha.git", sessions: []string{"s2"}},
				{name: "beta", remote: "git@github.com:user/beta.git", sessions: []string{"s3"}},
				{name: "zebra", remote: "git@github.com:user/zebra.git", sessions: []string{"s1"}},
			},
		},
		{
			name: "local repo comes first",
			sessions: []session.Session{
				{Name: "s1", Remote: "git@github.com:user/zebra.git"},
				{Name: "s2", Remote: "git@github.com:user/alpha.git"},
				{Name: "s3", Remote: "git@github.com:user/beta.git"},
			},
			localRemote: "git@github.com:user/beta.git",
			wantGroups: []struct {
				name     string
				remote   string
				sessions []string
			}{
				{name: "beta", remote: "git@github.com:user/beta.git", sessions: []string{"s3"}},
				{name: "alpha", remote: "git@github.com:user/alpha.git", sessions: []string{"s2"}},
				{name: "zebra", remote: "git@github.com:user/zebra.git", sessions: []string{"s1"}},
			},
		},
		{
			name: "sessions within groups sorted by name",
			sessions: []session.Session{
				{Name: "charlie", Remote: "git@github.com:user/repo.git"},
				{Name: "alpha", Remote: "git@github.com:user/repo.git"},
				{Name: "bravo", Remote: "git@github.com:user/repo.git"},
			},
			wantGroups: []struct {
				name     string
				remote   string
				sessions []string
			}{
				{name: "repo", remote: "git@github.com:user/repo.git", sessions: []string{"alpha", "bravo", "charlie"}},
			},
		},
		{
			name: "sessions with no remote grouped together",
			sessions: []session.Session{
				{Name: "s1", Remote: ""},
				{Name: "s2", Remote: "git@github.com:user/repo.git"},
				{Name: "s3", Remote: ""},
			},
			wantGroups: []struct {
				name     string
				remote   string
				sessions []string
			}{
				{name: "(no remote)", remote: "(no remote)", sessions: []string{"s1", "s3"}},
				{name: "repo", remote: "git@github.com:user/repo.git", sessions: []string{"s2"}},
			},
		},
		{
			name: "https remote format works",
			sessions: []session.Session{
				{Name: "s1", Remote: "https://github.com/user/my-repo.git"},
			},
			wantGroups: []struct {
				name     string
				remote   string
				sessions []string
			}{
				{name: "my-repo", remote: "https://github.com/user/my-repo.git", sessions: []string{"s1"}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			groups := GroupSessionsByRepo(tt.sessions, tt.localRemote)

			if len(tt.wantGroups) == 0 {
				assert.Empty(t, groups)
				return
			}

			require.Len(t, groups, len(tt.wantGroups))

			for i, want := range tt.wantGroups {
				got := groups[i]
				assert.Equal(t, want.name, got.Name, "group %d name mismatch", i)
				assert.Equal(t, want.remote, got.Remote, "group %d remote mismatch", i)

				gotNames := make([]string, len(got.Sessions))
				for j, s := range got.Sessions {
					gotNames[j] = s.Name
				}
				assert.Equal(t, want.sessions, gotNames, "group %d sessions mismatch", i)
			}
		})
	}
}

func TestExtractGroupName(t *testing.T) {
	tests := []struct {
		remote string
		want   string
	}{
		{"git@github.com:user/hive.git", "hive"},
		{"https://github.com/user/my-repo.git", "my-repo"},
		{"", "(no remote)"},
		{"(no remote)", "(no remote)"},
	}

	for _, tt := range tests {
		t.Run(tt.remote, func(t *testing.T) {
			got := extractGroupName(tt.remote)
			assert.Equal(t, tt.want, got)
		})
	}
}
