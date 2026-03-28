package main

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
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
		{"テスト", 6, "テスト"},   // 3 CJK chars = 6 columns, fits exactly
		{"テスト", 5, "テス…"},   // 2 CJK chars (4 cols) + ellipsis (1 col) = 5
		{"テスト", 2, "…"},      // only room for ellipsis
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
		{"テスト", 6},    // 3 CJK chars = 6 display columns
		{"aテストb", 8}, // 1 + 6 + 1 = 8
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

func TestPrintBranches(t *testing.T) {
	colorOn = false
	defer func() { colorOn = false }()

	branches := []Branch{
		{Name: "main", DisplayName: "main", ShortHash: "abc1234", Upstream: "origin/main", UpstreamRemote: "origin", IsHead: true, Subject: "Initial commit"},
		{Name: "feature", DisplayName: "feature", ShortHash: "def5678", Upstream: "origin/feature", UpstreamRemote: "origin", Ahead: 2, Subject: "Add feature"},
		{DisplayName: "dev", ShortHash: "ghi9012", IsRemote: true, RemoteName: "origin", Subject: "Remote dev"},
	}
	cw := computeWidths(branches, 100)

	var buf bytes.Buffer
	printBranches(&buf, branches, 100, cw)
	out := buf.String()

	// Each branch produces one line.
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("printBranches produced %d lines, want 3", len(lines))
	}

	// HEAD branch has * indicator and branch name.
	if !strings.Contains(lines[0], "*") || !strings.Contains(lines[0], "main") {
		t.Errorf("line 0: expected HEAD indicator and 'main', got %q", lines[0])
	}

	// Ahead branch shows deviation.
	if !strings.Contains(lines[1], "↑2") {
		t.Errorf("line 1: expected '↑2', got %q", lines[1])
	}

	// Remote branch shows remote name.
	if !strings.Contains(lines[2], "origin") || !strings.Contains(lines[2], "dev") {
		t.Errorf("line 2: expected remote 'origin' and 'dev', got %q", lines[2])
	}
}

func TestPrintBranchesNoDevColumn(t *testing.T) {
	colorOn = false
	defer func() { colorOn = false }()

	branches := []Branch{
		{Name: "main", DisplayName: "main", ShortHash: "abc1234", Upstream: "origin/main", UpstreamRemote: "origin", IsHead: true, Subject: "Initial commit"},
		{Name: "feature", DisplayName: "feature", ShortHash: "def5678", Subject: "Add feature"},
	}
	cw := computeWidths(branches, 100)

	if cw.dev != 0 {
		t.Fatalf("expected cw.dev=0, got %d", cw.dev)
	}

	var buf bytes.Buffer
	printBranches(&buf, branches, 100, cw)
	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")

	// With no deviation column, name should be followed by 2-space gap then remote/hash.
	// Should NOT have the extra gap from an empty dev column.
	for i, line := range lines {
		nameEnd := 2 + cw.name // indicator(2) + name column
		tail := line[nameEnd:]
		if strings.HasPrefix(tail, "    ") {
			t.Errorf("line %d: unexpected 4+ space gap after name column (empty dev column not omitted?): %q", i, line)
		}
	}
}

func TestRenderLine(t *testing.T) {
	colorOn = false
	defer func() { colorOn = false }()

	cw := colWidths{name: 20, dev: 4, remote: 10, hash: 7}

	// Local branch.
	b := &Branch{Name: "main", DisplayName: "main", ShortHash: "abc1234", Upstream: "origin/main", UpstreamRemote: "origin", IsHead: true, Subject: "Initial commit"}
	line := renderLine(b, cw, 80)
	plain := stripAnsi(line)
	if !strings.Contains(plain, "main") || !strings.Contains(plain, "abc1234") || !strings.Contains(plain, "origin") {
		t.Errorf("renderLine local: missing expected content in %q", plain)
	}

	// Remote branch.
	rb := &Branch{DisplayName: "feature", ShortHash: "def5678", IsRemote: true, RemoteName: "origin", Subject: "Remote feature"}
	rline := renderLine(rb, cw, 80)
	rplain := stripAnsi(rline)
	if !strings.Contains(rplain, "feature") || !strings.Contains(rplain, "origin") {
		t.Errorf("renderLine remote: missing expected content in %q", rplain)
	}

	// No deviation column: dev column should be omitted entirely.
	cwNoDev := colWidths{name: 20, dev: 0, remote: 10, hash: 7}
	nline := renderLine(b, cwNoDev, 80)
	nplain := stripAnsi(nline)
	if !strings.Contains(nplain, "main") || !strings.Contains(nplain, "origin") {
		t.Errorf("renderLine no-dev: missing expected content in %q", nplain)
	}
	// With dev=0, the line should be shorter (no dev column or its gaps).
	if runeLen(nplain) >= runeLen(plain) {
		t.Errorf("renderLine no-dev: expected shorter line, got %d >= %d", runeLen(nplain), runeLen(plain))
	}
}

func TestSearchTarget(t *testing.T) {
	tests := []struct {
		name string
		b    Branch
		want string
	}{
		{"local with upstream", Branch{DisplayName: "main", Upstream: "origin/main", UpstreamRemote: "origin", Name: "main"}, "main origin"},
		{"local diff upstream", Branch{DisplayName: "dev", Upstream: "origin/staging", UpstreamRemote: "origin", Name: "dev"}, "dev origin/staging"},
		{"local no upstream", Branch{DisplayName: "local-only"}, "local-only"},
		{"remote", Branch{DisplayName: "feature", IsRemote: true, RemoteName: "origin"}, "feature origin"},
	}
	for _, tt := range tests {
		got := searchTarget(&tt.b)
		if got != tt.want {
			t.Errorf("searchTarget(%s) = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestDevColorCode(t *testing.T) {
	tests := []struct {
		name string
		b    Branch
		want string
	}{
		{"remote", Branch{IsRemote: true}, ""},
		{"no upstream", Branch{}, ""},
		{"synced", Branch{Upstream: "origin/main"}, ""},
		{"ahead", Branch{Upstream: "origin/main", Ahead: 1}, cGreen},
		{"behind", Branch{Upstream: "origin/main", Behind: 2}, cYellow},
		{"diverged", Branch{Upstream: "origin/main", Ahead: 1, Behind: 2}, cBoldRed},
		{"gone", Branch{Upstream: "origin/main", Gone: true}, cBoldRed},
	}
	for _, tt := range tests {
		got := devColorCode(tt.b)
		if got != tt.want {
			t.Errorf("devColorCode(%s) = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestApplySelection(t *testing.T) {
	colorOn = true
	defer func() { colorOn = false }()

	line := "  hello world"
	result := applySelection(line, 20)

	// Should contain reverse video code.
	if !strings.Contains(result, cReverse) {
		t.Error("applySelection: missing reverse video code")
	}
	// Plain text content should be preserved.
	if !strings.Contains(stripAnsi(result), "hello world") {
		t.Error("applySelection: content lost")
	}
	// Should be padded to terminal width.
	plain := stripAnsi(result)
	if runeLen(plain) != 20 {
		t.Errorf("applySelection: width = %d, want 20", runeLen(plain))
	}

	// With colorOn = false, no ANSI codes should be emitted.
	colorOn = false
	noColor := applySelection(line, 20)
	if strings.Contains(noColor, "\033") {
		t.Error("applySelection with colorOn=false: should not contain ANSI codes")
	}
}

// --- Delete / TUI tests ---

func runeKey(r rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
}

func TestDeleteKeyGuards(t *testing.T) {
	colorOn = false
	defer func() { colorOn = false }()

	savedMerged := isBranchMerged
	defer func() { isBranchMerged = savedMerged }()
	isBranchMerged = func(string) bool { return true }

	items := []listItem{
		{branch: &Branch{Name: "main", DisplayName: "main", IsHead: true}},
		{branch: &Branch{Name: "feature", DisplayName: "feature"}},
		{branch: &Branch{DisplayName: "dev", IsRemote: true, RemoteName: "origin"}},
		{branch: &Branch{Name: "wt-branch", DisplayName: "wt-branch", WorktreePath: "wt"}},
	}
	m := tuiModel{
		items:  items,
		selIdx: []int{0, 1, 2, 3},
		tw:     80,
		th:     24,
	}

	// d on HEAD → error.
	m.cursor = 0
	result, _ := m.updateNormal(runeKey('d'))
	rm := result.(tuiModel)
	if !rm.statusIsErr || !strings.Contains(rm.statusMsg, "checked out") {
		t.Errorf("d on HEAD: statusIsErr=%v msg=%q", rm.statusIsErr, rm.statusMsg)
	}
	if rm.confirming {
		t.Error("d on HEAD: should not enter confirming")
	}

	// d on worktree branch → error.
	m.cursor = 3
	m.statusMsg = ""
	result, _ = m.updateNormal(runeKey('d'))
	rm = result.(tuiModel)
	if !rm.statusIsErr || !strings.Contains(rm.statusMsg, "checked out") {
		t.Errorf("d on worktree: statusIsErr=%v msg=%q", rm.statusIsErr, rm.statusMsg)
	}

	// d on remote → error with hint.
	m.cursor = 2
	m.statusMsg = ""
	result, _ = m.updateNormal(runeKey('d'))
	rm = result.(tuiModel)
	if !rm.statusIsErr || !strings.Contains(rm.statusMsg, "use D") {
		t.Errorf("d on remote: statusIsErr=%v msg=%q", rm.statusIsErr, rm.statusMsg)
	}

	// D on remote → confirmation.
	m.cursor = 2
	m.statusMsg = ""
	m.confirming = false
	result, _ = m.updateNormal(runeKey('D'))
	rm = result.(tuiModel)
	if !rm.confirming {
		t.Error("D on remote: should enter confirming")
	}

	// d on merged local branch → confirmation.
	m.cursor = 1
	m.statusMsg = ""
	result, _ = m.updateNormal(runeKey('d'))
	rm = result.(tuiModel)
	if !rm.confirming || rm.confirmForce {
		t.Errorf("d merged: confirming=%v confirmForce=%v", rm.confirming, rm.confirmForce)
	}

	// d on unmerged local branch → error with hint.
	isBranchMerged = func(string) bool { return false }
	m.cursor = 1
	m.statusMsg = ""
	m.confirming = false
	result, _ = m.updateNormal(runeKey('d'))
	rm = result.(tuiModel)
	if !rm.statusIsErr || !strings.Contains(rm.statusMsg, "not fully merged") {
		t.Errorf("d unmerged: statusIsErr=%v msg=%q", rm.statusIsErr, rm.statusMsg)
	}
	if rm.confirming {
		t.Error("d unmerged: should not enter confirming")
	}

	// D on unmerged local branch → still enters force confirmation.
	m.cursor = 1
	m.statusMsg = ""
	m.confirming = false
	result, _ = m.updateNormal(runeKey('D'))
	rm = result.(tuiModel)
	if !rm.confirming || !rm.confirmForce {
		t.Errorf("D unmerged: confirming=%v confirmForce=%v", rm.confirming, rm.confirmForce)
	}
}

func TestDeleteConfirmCancel(t *testing.T) {
	colorOn = false
	defer func() { colorOn = false }()

	items := []listItem{
		{branch: &Branch{Name: "main", DisplayName: "main", IsHead: true}},
		{branch: &Branch{Name: "feature", DisplayName: "feature"}},
	}
	m := tuiModel{
		items:      items,
		selIdx:     []int{0, 1},
		cursor:     1,
		confirming: true,
		tw:         80,
		th:         24,
	}

	result, _ := m.updateConfirm(runeKey('n'))
	rm := result.(tuiModel)
	if rm.confirming {
		t.Error("n should cancel confirmation")
	}
	if len(rm.selIdx) != 2 {
		t.Errorf("after cancel: %d selectable, want 2", len(rm.selIdx))
	}
}

func TestDeleteConfirmYes(t *testing.T) {
	colorOn = false
	defer func() { colorOn = false }()

	saved := gitBranchDelete
	defer func() { gitBranchDelete = saved }()
	gitBranchDelete = func(name string, force bool) (string, error) {
		return "", nil
	}

	branches := []Branch{
		{Name: "main", DisplayName: "main", IsHead: true, ShortHash: "abc1234"},
		{Name: "feature", DisplayName: "feature", ShortHash: "def5678"},
	}
	var items []listItem
	var selIdx []int
	for i := range branches {
		selIdx = append(selIdx, len(items))
		items = append(items, listItem{branch: &branches[i]})
	}

	m := tuiModel{
		allBranches: append([]Branch{}, branches...),
		items:       items,
		selIdx:      selIdx,
		cursor:      1,
		confirming:  true,
		tw:          80,
		th:          24,
	}

	result, _ := m.updateConfirm(runeKey('Y'))
	rm := result.(tuiModel)
	if rm.confirming {
		t.Error("Y should end confirmation")
	}
	if rm.statusIsErr {
		t.Errorf("expected success, got error: %q", rm.statusMsg)
	}
	if !strings.Contains(rm.statusMsg, "'feature'") {
		t.Errorf("status = %q, want quoted branch name", rm.statusMsg)
	}
	if !strings.Contains(rm.statusMsg, "def5678") {
		t.Errorf("status = %q, want short hash", rm.statusMsg)
	}
	if len(rm.selIdx) != 1 {
		t.Fatalf("after delete: %d selectable, want 1", len(rm.selIdx))
	}

	// Lowercase y should cancel, not confirm.
	m2 := m
	m2.confirming = true
	result2, _ := m2.updateConfirm(runeKey('y'))
	rm2 := result2.(tuiModel)
	if rm2.confirming {
		t.Error("lowercase y should cancel confirmation")
	}
	if len(rm2.selIdx) != 2 {
		t.Errorf("lowercase y should not delete: %d selectable, want 2", len(rm2.selIdx))
	}
}

func TestDeleteConfirmError(t *testing.T) {
	colorOn = false
	defer func() { colorOn = false }()

	saved := gitBranchDelete
	defer func() { gitBranchDelete = saved }()
	gitBranchDelete = func(name string, force bool) (string, error) {
		return "", fmt.Errorf("The branch '%s' is not fully merged.", name)
	}

	branches := []Branch{
		{Name: "main", DisplayName: "main", IsHead: true, ShortHash: "abc1234"},
		{Name: "feature", DisplayName: "feature", ShortHash: "def5678"},
	}
	var items []listItem
	var selIdx []int
	for i := range branches {
		selIdx = append(selIdx, len(items))
		items = append(items, listItem{branch: &branches[i]})
	}

	m := tuiModel{
		allBranches: append([]Branch{}, branches...),
		items:       items,
		selIdx:      selIdx,
		cursor:      1,
		confirming:  true,
		tw:          80,
		th:          24,
	}

	result, _ := m.updateConfirm(runeKey('Y'))
	rm := result.(tuiModel)
	if !rm.statusIsErr {
		t.Error("expected error status")
	}
	if !strings.Contains(rm.statusMsg, "not fully merged") {
		t.Errorf("error = %q, want 'not fully merged'", rm.statusMsg)
	}
	// Branch should NOT be removed on error.
	if len(rm.selIdx) != 2 {
		t.Errorf("after failed delete: %d selectable, want 2", len(rm.selIdx))
	}
}

func TestDeleteRemoteBranch(t *testing.T) {
	colorOn = false
	defer func() { colorOn = false }()

	savedRemote := gitRemoteBranchDelete
	defer func() { gitRemoteBranchDelete = savedRemote }()

	var deletedRemote, deletedBranch string
	gitRemoteBranchDelete = func(remote, branch string) error {
		deletedRemote = remote
		deletedBranch = branch
		return nil
	}

	branches := []Branch{
		{Name: "main", DisplayName: "main", IsHead: true, ShortHash: "abc1234"},
		{Name: "origin/feature", DisplayName: "feature", ShortHash: "def5678", IsRemote: true, RemoteName: "origin"},
	}
	var items []listItem
	var selIdx []int
	for i := range branches {
		selIdx = append(selIdx, len(items))
		items = append(items, listItem{branch: &branches[i]})
	}

	m := tuiModel{
		allBranches: append([]Branch{}, branches...),
		items:       items,
		selIdx:      selIdx,
		cursor:      1,
		confirming:  true,
		tw:          80,
		th:          24,
	}

	result, _ := m.updateConfirm(runeKey('Y'))
	rm := result.(tuiModel)
	if rm.statusIsErr {
		t.Errorf("expected success, got error: %q", rm.statusMsg)
	}
	if !strings.Contains(rm.statusMsg, "remote branch") {
		t.Errorf("status = %q, want 'remote branch'", rm.statusMsg)
	}
	if !strings.Contains(rm.statusMsg, "'origin/feature'") {
		t.Errorf("status = %q, want quoted name", rm.statusMsg)
	}
	if deletedRemote != "origin" || deletedBranch != "feature" {
		t.Errorf("called delete(%q, %q), want (origin, feature)", deletedRemote, deletedBranch)
	}
	if len(rm.selIdx) != 1 {
		t.Fatalf("after delete: %d selectable, want 1", len(rm.selIdx))
	}
}

func TestViewConfirmRemote(t *testing.T) {
	colorOn = false
	defer func() { colorOn = false }()

	items := []listItem{
		{branch: &Branch{Name: "origin/feature", DisplayName: "feature", ShortHash: "def5678", IsRemote: true, RemoteName: "origin"}},
	}
	m := tuiModel{
		items:      items,
		selIdx:     []int{0},
		cursor:     0,
		confirming: true,
		tw:         80,
		th:         10,
		cw:         colWidths{name: 20, dev: 0, remote: 10, hash: 7},
	}

	view := m.View()
	if !strings.Contains(view, "Delete 'origin/feature' from remote? [Y/n]") {
		t.Errorf("view should show remote delete confirmation, got:\n%s", view)
	}
}

func TestRemoveCurrent(t *testing.T) {
	colorOn = false
	defer func() { colorOn = false }()

	branches := []Branch{
		{Name: "main", DisplayName: "main", IsHead: true, ShortHash: "abc1234"},
		{Name: "feature", DisplayName: "feature", ShortHash: "def5678"},
		{Name: "bugfix", DisplayName: "bugfix", ShortHash: "ghi9012"},
	}
	var items []listItem
	var selIdx []int
	for i := range branches {
		selIdx = append(selIdx, len(items))
		items = append(items, listItem{branch: &branches[i]})
	}

	m := tuiModel{
		allBranches: append([]Branch{}, branches...),
		items:       items,
		selIdx:      selIdx,
		cursor:      1, // pointing at "feature"
		tw:          80,
		th:          24,
	}

	m.removeCurrent()

	if len(m.selIdx) != 2 {
		t.Fatalf("after remove: %d selectable, want 2", len(m.selIdx))
	}
	if m.cursor != 1 {
		t.Errorf("cursor = %d, want 1", m.cursor)
	}

	var names []string
	for _, idx := range m.selIdx {
		names = append(names, m.items[idx].branch.DisplayName)
	}
	want := []string{"main", "bugfix"}
	for i, w := range want {
		if names[i] != w {
			t.Errorf("branch %d = %q, want %q", i, names[i], w)
		}
	}
}

func TestRemoveCurrentLast(t *testing.T) {
	colorOn = false
	defer func() { colorOn = false }()

	branches := []Branch{
		{Name: "main", DisplayName: "main", IsHead: true, ShortHash: "abc1234"},
		{Name: "feature", DisplayName: "feature", ShortHash: "def5678"},
	}
	var items []listItem
	var selIdx []int
	for i := range branches {
		selIdx = append(selIdx, len(items))
		items = append(items, listItem{branch: &branches[i]})
	}

	m := tuiModel{
		allBranches: append([]Branch{}, branches...),
		items:       items,
		selIdx:      selIdx,
		cursor:      1, // last item
		tw:          80,
		th:          24,
	}

	m.removeCurrent()

	if len(m.selIdx) != 1 {
		t.Fatalf("after remove: %d selectable, want 1", len(m.selIdx))
	}
	if m.cursor != 0 {
		t.Errorf("cursor = %d, want 0", m.cursor)
	}
	if m.items[m.selIdx[0]].branch.Name != "main" {
		t.Errorf("remaining = %q, want 'main'", m.items[m.selIdx[0]].branch.Name)
	}
}

func TestViewConfirm(t *testing.T) {
	colorOn = false
	defer func() { colorOn = false }()

	items := []listItem{
		{branch: &Branch{Name: "main", DisplayName: "main", IsHead: true, ShortHash: "abc1234"}},
		{branch: &Branch{Name: "feature", DisplayName: "feature", ShortHash: "def5678"}},
	}
	m := tuiModel{
		items:      items,
		selIdx:     []int{0, 1},
		cursor:     1,
		confirming: true,
		tw:         80,
		th:         10,
		cw:         colWidths{name: 20, dev: 4, remote: 10, hash: 7},
	}

	view := m.View()
	if !strings.Contains(view, "Delete 'feature'? [Y/n]") {
		t.Errorf("view should show delete confirmation, got:\n%s", view)
	}

	m.confirmForce = true
	view = m.View()
	if !strings.Contains(view, "Force delete 'feature'? [Y/n]") {
		t.Errorf("view should show force delete confirmation, got:\n%s", view)
	}
}

func TestViewStatus(t *testing.T) {
	colorOn = false
	defer func() { colorOn = false }()

	items := []listItem{
		{branch: &Branch{Name: "main", DisplayName: "main", IsHead: true, ShortHash: "abc1234"}},
	}
	m := tuiModel{
		items:     items,
		selIdx:    []int{0},
		statusMsg: "Deleted branch feature (was abc1234).",
		tw:        80,
		th:        10,
		cw:        colWidths{name: 20, dev: 4, remote: 10, hash: 7},
	}

	view := m.View()
	if !strings.Contains(view, "Deleted branch feature") {
		t.Errorf("view should show status, got:\n%s", view)
	}
}

func TestStatusClearsOnKey(t *testing.T) {
	colorOn = false
	defer func() { colorOn = false }()

	items := []listItem{
		{branch: &Branch{Name: "main", DisplayName: "main", IsHead: true}},
		{branch: &Branch{Name: "feature", DisplayName: "feature"}},
	}
	m := tuiModel{
		items:       items,
		selIdx:      []int{0, 1},
		cursor:      0,
		statusMsg:   "some message",
		statusIsErr: false,
		tw:          80,
		th:          24,
	}

	result, _ := m.updateNormal(runeKey('j'))
	rm := result.(tuiModel)
	if rm.statusMsg != "" {
		t.Errorf("status should be cleared, got %q", rm.statusMsg)
	}
}

func TestSearchEnterReturnsToNormal(t *testing.T) {
	colorOn = false
	defer func() { colorOn = false }()

	items := []listItem{
		{branch: &Branch{Name: "main", DisplayName: "main", IsHead: true}},
		{branch: &Branch{Name: "feature", DisplayName: "feature"}},
		{branch: &Branch{Name: "bugfix", DisplayName: "bugfix"}},
	}
	m := tuiModel{
		items:       items,
		selIdx:      []int{0, 1, 2},
		searching:   true,
		query:       "feat",
		filteredIdx: []int{1}, // only "feature" matches
		cursor:      0,        // first (only) match
		tw:          80,
		th:          24,
	}

	result, cmd := m.updateSearch(tea.KeyMsg{Type: tea.KeyEnter})
	rm := result.(tuiModel)

	// Should exit search mode, not quit.
	if cmd != nil {
		t.Error("enter in search should not produce a quit command")
	}
	if rm.searching {
		t.Error("should exit search mode")
	}
	if rm.chosen != nil {
		t.Error("should not set chosen (no checkout)")
	}

	// Cursor should be on "feature" (selIdx position 1).
	if rm.cursor != 1 {
		t.Errorf("cursor = %d, want 1 (feature)", rm.cursor)
	}
	if rm.query != "" {
		t.Errorf("query should be cleared, got %q", rm.query)
	}
}

func TestPagerCommand(t *testing.T) {
	// Save and restore env vars.
	for _, key := range []string{"GIT_PAGER", "PAGER"} {
		if orig, ok := os.LookupEnv(key); ok {
			defer os.Setenv(key, orig)
		} else {
			defer os.Unsetenv(key)
		}
	}

	tests := []struct {
		name     string
		setup    func()
		wantName string
		wantArgs []string
	}{
		{
			name: "GIT_PAGER set",
			setup: func() {
				os.Setenv("GIT_PAGER", "bat")
				os.Unsetenv("PAGER")
			},
			wantName: "sh",
			wantArgs: []string{"-c", "bat"},
		},
		{
			name: "GIT_PAGER empty disables pager",
			setup: func() {
				os.Setenv("GIT_PAGER", "")
				os.Setenv("PAGER", "more")
			},
			wantName: "",
			wantArgs: nil,
		},
		{
			name: "PAGER fallback",
			setup: func() {
				os.Unsetenv("GIT_PAGER")
				os.Setenv("PAGER", "more")
			},
			wantName: "sh",
			wantArgs: []string{"-c", "more"},
		},
		{
			name: "PAGER empty disables pager",
			setup: func() {
				os.Unsetenv("GIT_PAGER")
				os.Setenv("PAGER", "")
			},
			wantName: "",
			wantArgs: nil,
		},
		{
			name: "default is less",
			setup: func() {
				os.Unsetenv("GIT_PAGER")
				os.Unsetenv("PAGER")
			},
			wantName: "less",
			wantArgs: []string{"-RFX"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			name, args := pagerCommand()
			if name != tt.wantName {
				t.Errorf("name = %q, want %q", name, tt.wantName)
			}
			if fmt.Sprint(args) != fmt.Sprint(tt.wantArgs) {
				t.Errorf("args = %v, want %v", args, tt.wantArgs)
			}
		})
	}
}

func TestNavigationKeys(t *testing.T) {
	colorOn = false
	defer func() { colorOn = false }()

	items := make([]listItem, 20)
	selIdx := make([]int, 20)
	for i := range items {
		items[i] = listItem{branch: &Branch{Name: fmt.Sprintf("b%d", i), DisplayName: fmt.Sprintf("b%d", i)}}
		selIdx[i] = i
	}
	items[0].branch.IsHead = true

	base := tuiModel{
		items:  items,
		selIdx: selIdx,
		tw:     80,
		th:     10, // viewHeight = 10 - 2 = 8
		cursor: 5,
	}

	// g → top
	m := base
	result, _ := m.updateNormal(runeKey('g'))
	rm := result.(tuiModel)
	if rm.cursor != 0 {
		t.Errorf("g: cursor = %d, want 0", rm.cursor)
	}

	// G → bottom
	m = base
	result, _ = m.updateNormal(runeKey('G'))
	rm = result.(tuiModel)
	if rm.cursor != 19 {
		t.Errorf("G: cursor = %d, want 19", rm.cursor)
	}

	// pgdown from cursor 5 with viewHeight 8 → 13
	m = base
	result, _ = m.updateNormal(tea.KeyMsg{Type: tea.KeyPgDown})
	rm = result.(tuiModel)
	if rm.cursor != 13 {
		t.Errorf("pgdown: cursor = %d, want 13", rm.cursor)
	}

	// pgup from cursor 5 with viewHeight 8 → 0 (clamped)
	m = base
	result, _ = m.updateNormal(tea.KeyMsg{Type: tea.KeyPgUp})
	rm = result.(tuiModel)
	if rm.cursor != 0 {
		t.Errorf("pgup: cursor = %d, want 0", rm.cursor)
	}

	// pgdown at end → stays at 19
	m = base
	m.cursor = 19
	result, _ = m.updateNormal(tea.KeyMsg{Type: tea.KeyPgDown})
	rm = result.(tuiModel)
	if rm.cursor != 19 {
		t.Errorf("pgdown at end: cursor = %d, want 19", rm.cursor)
	}
}
