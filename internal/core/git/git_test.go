package git

import "testing"

func TestExtractOwnerRepo(t *testing.T) {
	tests := []struct {
		remote    string
		wantOwner string
		wantRepo  string
	}{
		{"git@github.com:hay-kot/hive.git", "hay-kot", "hive"},
		{"https://github.com/hay-kot/hive.git", "hay-kot", "hive"},
		{"git@github.com:hay-kot/hive", "hay-kot", "hive"},
		{"https://github.com/hay-kot/hive", "hay-kot", "hive"},
		{"git@gitlab.com:org/subgroup/repo.git", "subgroup", "repo"},
		{"https://gitlab.com/org/subgroup/repo.git", "subgroup", "repo"},
		{"invalid", "", ""},
		{"", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.remote, func(t *testing.T) {
			owner, repo := ExtractOwnerRepo(tt.remote)
			if owner != tt.wantOwner || repo != tt.wantRepo {
				t.Errorf("ExtractOwnerRepo(%q) = (%q, %q), want (%q, %q)",
					tt.remote, owner, repo, tt.wantOwner, tt.wantRepo)
			}
		})
	}
}

func TestExtractRepoName(t *testing.T) {
	tests := []struct {
		remote   string
		wantRepo string
	}{
		{"git@github.com:hay-kot/hive.git", "hive"},
		{"https://github.com/hay-kot/hive.git", "hive"},
		{"git@github.com:hay-kot/hive", "hive"},
		{"https://github.com/hay-kot/hive", "hive"},
	}

	for _, tt := range tests {
		t.Run(tt.remote, func(t *testing.T) {
			repo := ExtractRepoName(tt.remote)
			if repo != tt.wantRepo {
				t.Errorf("ExtractRepoName(%q) = %q, want %q", tt.remote, repo, tt.wantRepo)
			}
		})
	}
}
