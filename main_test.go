package main

import "testing"

func TestTrunc(t *testing.T) {
	tests := []struct {
		s    string
		max  int
		want string
	}{
		{"hello", 10, "hello"},
		{"hello", 5, "hello"},
		{"hello", 4, "hel…"},
		{"hello", 1, "…"},
		{"", 5, ""},
		{"abc", 3, "abc"},
		{"abcd", 3, "ab…"},
		{"hello", 0, ""},
		{"hello", -1, ""},
	}
	for _, tt := range tests {
		got := trunc(tt.s, tt.max)
		if got != tt.want {
			t.Errorf("trunc(%q, %d) = %q, want %q", tt.s, tt.max, got, tt.want)
		}
	}
}

func TestRuneLen(t *testing.T) {
	tests := []struct {
		s    string
		want int
	}{
		{"hello", 5},
		{"", 0},
		{"↑3↓2", 4},
		{"…", 1},
	}
	for _, tt := range tests {
		got := runeLen(tt.s)
		if got != tt.want {
			t.Errorf("runeLen(%q) = %d, want %d", tt.s, got, tt.want)
		}
	}
}

func TestFuzzyMatch(t *testing.T) {
	tests := []struct {
		query, target string
		want          bool
	}{
		{"", "anything", true},
		{"abc", "aXbXc", true},
		{"abc", "ABC", true},
		{"pla14", "pla-1474-security", true},
		{"xyz", "abc", false},
		{"abc", "ab", false},
		{"ori", "origin", true},
		{"fix", "richarddodson/fix-flaky", true},
	}
	for _, tt := range tests {
		got := fuzzyMatch(tt.query, tt.target)
		if got != tt.want {
			t.Errorf("fuzzyMatch(%q, %q) = %v, want %v", tt.query, tt.target, got, tt.want)
		}
	}
}

func TestStripAnsi(t *testing.T) {
	tests := []struct {
		s    string
		want string
	}{
		{"hello", "hello"},
		{"\033[1;32mgreen\033[0m", "green"},
		{"\033[31mred\033[0m \033[34mblue\033[0m", "red blue"},
		{"", ""},
		{"no escapes", "no escapes"},
	}
	for _, tt := range tests {
		got := stripAnsi(tt.s)
		if got != tt.want {
			t.Errorf("stripAnsi(%q) = %q, want %q", tt.s, got, tt.want)
		}
	}
}

func TestDevPlain(t *testing.T) {
	tests := []struct {
		name string
		b    Branch
		want string
	}{
		{"remote", Branch{IsRemote: true}, ""},
		{"no upstream", Branch{}, ""},
		{"synced", Branch{Upstream: "origin/main"}, ""},
		{"ahead", Branch{Upstream: "origin/main", Ahead: 3}, "↑3"},
		{"behind", Branch{Upstream: "origin/main", Behind: 2}, "↓2"},
		{"diverged", Branch{Upstream: "origin/main", Ahead: 1, Behind: 4}, "↑1↓4"},
		{"gone", Branch{Upstream: "origin/main", Gone: true}, "gone"},
	}
	for _, tt := range tests {
		got := devPlain(tt.b)
		if got != tt.want {
			t.Errorf("devPlain(%s) = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestTrackRef(t *testing.T) {
	tests := []struct {
		name string
		b    Branch
		want string
	}{
		{
			"same name",
			Branch{Name: "main", Upstream: "origin/main", UpstreamRemote: "origin"},
			"origin",
		},
		{
			"different name",
			Branch{Name: "main", Upstream: "origin/master", UpstreamRemote: "origin"},
			"origin/master",
		},
		{
			"local tracking",
			Branch{Name: "develop", Upstream: "staging"},
			"staging",
		},
	}
	for _, tt := range tests {
		got := trackRef(tt.b)
		if got != tt.want {
			t.Errorf("trackRef(%s) = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestSortBranches(t *testing.T) {
	branches := []Branch{
		{DisplayName: "zebra"},
		{DisplayName: "alpha", WorktreePath: "wt"},
		{DisplayName: "main", IsHead: true},
		{DisplayName: "beta"},
	}
	sortBranches(branches)

	want := []string{"main", "alpha", "beta", "zebra"}
	for i, w := range want {
		if branches[i].DisplayName != w {
			t.Errorf("sortBranches[%d] = %q, want %q", i, branches[i].DisplayName, w)
		}
	}
}

func TestRemotePlain(t *testing.T) {
	tests := []struct {
		name string
		b    Branch
		want string
	}{
		{"remote branch", Branch{IsRemote: true, RemoteName: "origin"}, "origin"},
		{"no upstream", Branch{}, "local"},
		{"same name tracking", Branch{Name: "main", Upstream: "origin/main", UpstreamRemote: "origin"}, "origin"},
		{"different name tracking", Branch{Name: "main", Upstream: "origin/master", UpstreamRemote: "origin"}, "origin/master"},
	}
	for _, tt := range tests {
		got := remotePlain(tt.b)
		if got != tt.want {
			t.Errorf("remotePlain(%s) = %q, want %q", tt.name, got, tt.want)
		}
	}
}
