package main

import (
	"strings"
	"testing"
)

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

func TestComputeWidths(t *testing.T) {
	branches := []Branch{
		{Name: "long-branch", DisplayName: "a-very-long-branch-name-that-exceeds-fifty-characters-easily", ShortHash: "abc1234", Upstream: "origin/long-branch", UpstreamRemote: "origin"},
		{Name: "short", DisplayName: "short", ShortHash: "def5678901", Upstream: "origin/short", UpstreamRemote: "origin"},
	}

	cw := computeWidths(branches, 120)

	// Name should be capped at 50.
	if cw.name > 50 {
		t.Errorf("name width %d exceeds cap of 50", cw.name)
	}
	if cw.name < 20 {
		t.Errorf("name width %d below minimum of 20", cw.name)
	}
	// Hash should match the longest hash.
	if cw.hash != 10 {
		t.Errorf("hash width = %d, want 10", cw.hash)
	}
	// Remote should be "origin" (6 chars) since both track same-name remotes.
	if cw.remote != 6 {
		t.Errorf("remote width = %d, want 6", cw.remote)
	}

	// Remote cap at 20.
	longRemote := []Branch{
		{DisplayName: "main", ShortHash: "abc1234", Upstream: "origin/a-very-different-name", UpstreamRemote: "origin", Name: "main"},
	}
	cw2 := computeWidths(longRemote, 120)
	if cw2.remote > 20 {
		t.Errorf("remote width %d exceeds cap of 20", cw2.remote)
	}

	// Narrow terminal: name floors at 20.
	cw3 := computeWidths(branches, 40)
	if cw3.name < 20 {
		t.Errorf("narrow terminal: name width %d below minimum of 20", cw3.name)
	}
}

func TestParseBranches(t *testing.T) {
	raw := strings.Join([]string{
		"*\trefs/heads/main\tmain\tabc1234\torigin/main\t\t\tInitial commit",
		" \trefs/heads/feature\tfeature\tdef5678\torigin/feature\tahead 3\t\tAdd feature",
		" \trefs/heads/old\told\tghi9012\torigin/old\tgone\t\tOld branch",
		" \trefs/heads/behind\tbehind\tpqr1234\torigin/behind\tbehind 5\t\tBehind branch",
		" \trefs/heads/diverged\tdiverged\tstu5678\torigin/diverged\tahead 2, behind 3\t\tDiverged branch",
		" \trefs/heads/wt\twt\tjkl3456\t\t\t/home/user/worktrees/wt\tWorktree branch",
		" \trefs/remotes/origin/dev\torigin/dev\tmno7890\t\t\t\tRemote dev",
		" \trefs/remotes/origin/HEAD\torigin/HEAD\tabc1234\t\t\t\t",
	}, "\n")

	branches := parseBranches(raw)

	if len(branches) != 7 {
		t.Fatalf("parseBranches returned %d branches, want 7 (origin/HEAD should be skipped)", len(branches))
	}

	// HEAD branch.
	if !branches[0].IsHead || branches[0].Name != "main" {
		t.Errorf("branch 0: want HEAD main, got IsHead=%v Name=%q", branches[0].IsHead, branches[0].Name)
	}

	// Ahead tracking.
	if branches[1].Ahead != 3 || branches[1].Behind != 0 {
		t.Errorf("branch 1: want ahead=3 behind=0, got ahead=%d behind=%d", branches[1].Ahead, branches[1].Behind)
	}

	// Gone upstream.
	if !branches[2].Gone {
		t.Error("branch 2: want Gone=true")
	}

	// Behind tracking.
	if branches[3].Ahead != 0 || branches[3].Behind != 5 {
		t.Errorf("branch 3: want ahead=0 behind=5, got ahead=%d behind=%d", branches[3].Ahead, branches[3].Behind)
	}

	// Diverged (ahead + behind).
	if branches[4].Ahead != 2 || branches[4].Behind != 3 {
		t.Errorf("branch 4: want ahead=2 behind=3, got ahead=%d behind=%d", branches[4].Ahead, branches[4].Behind)
	}

	// Worktree (non-HEAD gets basename).
	if branches[5].WorktreePath != "wt" {
		t.Errorf("branch 5: want WorktreePath=%q, got %q", "wt", branches[5].WorktreePath)
	}

	// Remote branch.
	b := branches[6]
	if !b.IsRemote || b.RemoteName != "origin" || b.DisplayName != "dev" {
		t.Errorf("branch 6: want remote origin/dev, got IsRemote=%v RemoteName=%q DisplayName=%q", b.IsRemote, b.RemoteName, b.DisplayName)
	}
}

func TestDevColored(t *testing.T) {
	colorOn = true
	defer func() { colorOn = false }()

	tests := []struct {
		name    string
		b       Branch
		wantSub string // substring that should appear in the colored output
	}{
		{"remote", Branch{IsRemote: true}, ""},
		{"no upstream", Branch{}, ""},
		{"synced", Branch{Upstream: "origin/main"}, ""},
		{"ahead", Branch{Upstream: "origin/main", Ahead: 2}, "↑2"},
		{"behind", Branch{Upstream: "origin/main", Behind: 5}, "↓5"},
		{"diverged", Branch{Upstream: "origin/main", Ahead: 1, Behind: 3}, "↑1↓3"},
		{"gone", Branch{Upstream: "origin/main", Gone: true}, "gone"},
	}
	for _, tt := range tests {
		got := devColored(tt.b)
		plain := stripAnsi(got)
		if plain != tt.wantSub {
			t.Errorf("devColored(%s) plain = %q, want %q", tt.name, plain, tt.wantSub)
		}
	}
}

func TestRemoteColored(t *testing.T) {
	colorOn = true
	defer func() { colorOn = false }()

	tests := []struct {
		name    string
		b       Branch
		wantSub string
	}{
		{"remote branch", Branch{IsRemote: true, RemoteName: "origin"}, "origin"},
		{"no upstream", Branch{}, "local"},
		{"same name", Branch{Name: "main", Upstream: "origin/main", UpstreamRemote: "origin"}, "origin"},
		{"diff name", Branch{Name: "main", Upstream: "origin/master", UpstreamRemote: "origin"}, "origin/master"},
	}
	for _, tt := range tests {
		got := remoteColored(tt.b, 20)
		plain := stripAnsi(got)
		if plain != tt.wantSub {
			t.Errorf("remoteColored(%s) plain = %q, want %q", tt.name, plain, tt.wantSub)
		}
	}
}
