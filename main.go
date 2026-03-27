package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/term"
)

const version = "0.1.0"

// ANSI color codes.
const (
	cReset    = "\033[0m"
	cBold     = "\033[1m"
	cDim      = "\033[2m"
	cRed      = "\033[31m"
	cGreen    = "\033[32m"
	cYellow   = "\033[33m"
	cCyan     = "\033[36m"
	cBoldGrn  = "\033[1;32m"
	cBoldCyan = "\033[1;36m"
	cBoldRed  = "\033[1;31m"
)

var colorOn bool

func clr(code, text string) string {
	if !colorOn || text == "" {
		return text
	}
	return code + text + cReset
}

// Branch holds parsed information about a single git ref.
type Branch struct {
	Name           string // refname:short
	FullRef        string // full refname for classification
	ShortHash      string
	Upstream       string // upstream:short (e.g. "origin/main")
	UpstreamRemote string // just the remote name (e.g. "origin")
	Ahead          int
	Behind         int
	Gone           bool   // upstream deleted
	Subject        string // commit message first line
	IsHead         bool   // checked out in current worktree
	WorktreePath   string // basename of linked worktree path (empty for HEAD or non-worktree)
	IsRemote       bool
	RemoteName     string // for remotes: "origin", etc.
	DisplayName    string // name used for display (strips remote prefix for remotes)
}

func main() {
	showAll := flag.Bool("a", false, "include remote-tracking branches")
	flag.BoolVar(showAll, "all", false, "include remote-tracking branches")
	noColor := flag.Bool("no-color", false, "disable colored output")
	showVer := flag.Bool("version", false, "show version")
	flag.BoolVar(showVer, "v", false, "show version")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: git better-branch [flags]\n\nA better git branch viewer.\n\nFlags:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if *showVer {
		fmt.Printf("git-better-branch %s\n", version)
		return
	}

	colorOn = term.IsTerminal(int(os.Stdout.Fd()))
	if *noColor || os.Getenv("NO_COLOR") != "" {
		colorOn = false
	}

	branches, err := loadBranches(*showAll)
	if err != nil {
		fmt.Fprintf(os.Stderr, "gbb: %v\n", err)
		os.Exit(1)
	}
	if len(branches) == 0 {
		return
	}

	tw := getTermWidth()

	var local, remote []Branch
	for _, b := range branches {
		if b.IsRemote {
			remote = append(remote, b)
		} else {
			local = append(local, b)
		}
	}

	sortBranches(local)
	printBranches(local, tw)

	if *showAll && len(remote) > 0 {
		groups := groupByRemote(remote)
		for _, g := range groups {
			fmt.Println()
			fmt.Println(clr(cBold, fmt.Sprintf("remote/%s:", g.name)))
			sortBranches(g.branches)
			printBranches(g.branches, tw)
		}
	}
}

// -------------------------------------------------------------------
// Data loading
// -------------------------------------------------------------------

func loadBranches(includeRemotes bool) ([]Branch, error) {
	// Tab-separated fields. Subject is last so any tabs it contains are harmless
	// (SplitN caps the split count, so the tail stays in the last field).
	format := strings.Join([]string{
		"%(HEAD)",                       // 0: * or space
		"%(refname)",                    // 1: full ref
		"%(refname:short)",              // 2: short ref
		"%(objectname:short)",           // 3: abbrev hash
		"%(upstream:short)",             // 4: upstream short
		"%(upstream:track,nobracket)",   // 5: ahead/behind/gone
		"%(worktreepath)",               // 6: worktree path
		"%(subject)",                    // 7: commit subject
	}, "\t")

	args := []string{"for-each-ref", "--format", format, "refs/heads/"}
	if includeRemotes {
		args = append(args, "refs/remotes/")
	}

	cmd := exec.Command("git", args...)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git for-each-ref: %w", err)
	}

	raw := strings.TrimRight(string(out), "\n")
	if raw == "" {
		return nil, nil
	}

	var branches []Branch
	for _, line := range strings.Split(raw, "\n") {
		fields := strings.SplitN(line, "\t", 8)
		if len(fields) < 8 {
			continue
		}

		fullRef := fields[1]
		isRemote := strings.HasPrefix(fullRef, "refs/remotes/")

		// Skip symbolic refs like origin/HEAD.
		if isRemote && strings.HasSuffix(fullRef, "/HEAD") {
			continue
		}

		b := Branch{
			IsHead:    fields[0] == "*",
			FullRef:   fullRef,
			Name:      fields[2],
			ShortHash: fields[3],
			Upstream:  fields[4],
			Subject:   fields[7],
			IsRemote:  isRemote,
		}

		// Upstream remote name.
		if b.Upstream != "" {
			if idx := strings.Index(b.Upstream, "/"); idx > 0 {
				b.UpstreamRemote = b.Upstream[:idx]
			}
		}

		// Remote branch: extract remote name and display name.
		if isRemote {
			stripped := strings.TrimPrefix(fullRef, "refs/remotes/")
			if idx := strings.Index(stripped, "/"); idx > 0 {
				b.RemoteName = stripped[:idx]
				b.DisplayName = stripped[idx+1:]
			} else {
				b.RemoteName = stripped
				b.DisplayName = stripped
			}
		} else {
			b.DisplayName = b.Name
		}

		// Tracking info.
		track := fields[5]
		switch {
		case track == "gone":
			b.Gone = true
		case track != "":
			for _, part := range strings.Split(track, ", ") {
				part = strings.TrimSpace(part)
				if strings.HasPrefix(part, "ahead ") {
					b.Ahead, _ = strconv.Atoi(strings.TrimPrefix(part, "ahead "))
				} else if strings.HasPrefix(part, "behind ") {
					b.Behind, _ = strconv.Atoi(strings.TrimPrefix(part, "behind "))
				}
			}
		}

		// Worktree: store basename only, skip for current HEAD.
		if fields[6] != "" && !b.IsHead {
			b.WorktreePath = filepath.Base(fields[6])
		}

		branches = append(branches, b)
	}

	return branches, nil
}

// -------------------------------------------------------------------
// Grouping and sorting
// -------------------------------------------------------------------

type remoteGroup struct {
	name     string
	branches []Branch
}

func groupByRemote(branches []Branch) []remoteGroup {
	m := make(map[string][]Branch)
	var order []string
	for _, b := range branches {
		if _, exists := m[b.RemoteName]; !exists {
			order = append(order, b.RemoteName)
		}
		m[b.RemoteName] = append(m[b.RemoteName], b)
	}
	groups := make([]remoteGroup, 0, len(order))
	for _, name := range order {
		groups = append(groups, remoteGroup{name: name, branches: m[name]})
	}
	return groups
}

func sortBranches(branches []Branch) {
	sort.SliceStable(branches, func(i, j int) bool {
		bi, bj := branches[i], branches[j]
		if bi.IsHead != bj.IsHead {
			return bi.IsHead
		}
		wi, wj := bi.WorktreePath != "", bj.WorktreePath != ""
		if wi != wj {
			return wi
		}
		return bi.DisplayName < bj.DisplayName
	})
}

// -------------------------------------------------------------------
// Display
// -------------------------------------------------------------------

func printBranches(branches []Branch, tw int) {
	if len(branches) == 0 {
		return
	}

	// Measure columns.
	maxName, maxHash, maxTrack := 0, 0, 0
	for _, b := range branches {
		if n := runeLen(b.DisplayName); n > maxName {
			maxName = n
		}
		if n := runeLen(b.ShortHash); n > maxHash {
			maxHash = n
		}
		if n := runeLen(trackPlain(b)); n > maxTrack {
			maxTrack = n
		}
	}

	// Cap name column so the line still fits.
	// Layout: indicator(2) + name + gap(2) + hash + gap(1) + track + gap(2) + subject(>=20)
	nameCap := tw - 2 - 2 - maxHash - 1 - maxTrack - 2 - 20
	if nameCap < 20 {
		nameCap = 20
	}
	if nameCap > 50 {
		nameCap = 50
	}
	if maxName > nameCap {
		maxName = nameCap
	}
	if maxTrack > 16 {
		maxTrack = 16
	}

	for _, b := range branches {
		// Indicator.
		var ind string
		switch {
		case b.IsHead:
			ind = clr(cBoldGrn, "*") + " "
		case b.WorktreePath != "":
			ind = clr(cBoldCyan, "+") + " "
		default:
			ind = "  "
		}

		// Name: truncate + pad.
		name := trunc(b.DisplayName, maxName)
		namePad := strings.Repeat(" ", maxName-runeLen(name))
		switch {
		case b.IsHead:
			name = clr(cBoldGrn, name+namePad)
		case b.WorktreePath != "":
			name = clr(cBoldCyan, name+namePad)
		default:
			name = name + namePad
		}

		// Hash: pad.
		hash := b.ShortHash + strings.Repeat(" ", maxHash-runeLen(b.ShortHash))
		hash = clr(cDim, hash)

		// Tracking: colored + pad.
		tp := trackPlain(b)
		tc := trackColored(b)
		tPad := strings.Repeat(" ", maxTrack-runeLen(tp))

		// Worktree tag (appended after subject).
		var wtTag string
		var wtPlainLen int
		if b.WorktreePath != "" {
			wtTag = " " + clr(cCyan, "["+b.WorktreePath+"]")
			wtPlainLen = 3 + runeLen(b.WorktreePath) // " [" + name + "]"
		}

		// Subject: fill remaining width.
		used := 2 + maxName + 2 + maxHash + 1 + maxTrack + 2 + wtPlainLen
		subWidth := tw - used
		if subWidth < 10 {
			subWidth = 10
		}
		subject := trunc(b.Subject, subWidth)

		fmt.Printf("%s%s  %s %s%s  %s%s\n", ind, name, hash, tc, tPad, subject, wtTag)
	}
}

// trackRef returns the remote display string: just the remote name if the
// upstream branch matches the local name, or the full upstream ref if they differ.
func trackRef(b Branch) string {
	if b.UpstreamRemote == "" {
		// Local-to-local tracking (no remote prefix).
		return b.Upstream
	}
	upstreamBranch := strings.TrimPrefix(b.Upstream, b.UpstreamRemote+"/")
	if upstreamBranch == b.Name {
		return b.UpstreamRemote
	}
	return b.Upstream
}

// trackPlain returns the tracking status as plain text (for width measurement).
func trackPlain(b Branch) string {
	if b.IsRemote {
		return ""
	}
	if b.Gone {
		return "gone"
	}
	if b.Upstream == "" {
		return "local"
	}
	r := trackRef(b)
	switch {
	case b.Ahead == 0 && b.Behind == 0:
		return "= " + r
	case b.Ahead > 0 && b.Behind == 0:
		return fmt.Sprintf("↑%d %s", b.Ahead, r)
	case b.Ahead == 0 && b.Behind > 0:
		return fmt.Sprintf("↓%d %s", b.Behind, r)
	default:
		return fmt.Sprintf("↑%d↓%d %s", b.Ahead, b.Behind, r)
	}
}

// trackColored returns the tracking status with ANSI colors.
func trackColored(b Branch) string {
	if b.IsRemote {
		return ""
	}
	if b.Gone {
		return clr(cBoldRed, "gone")
	}
	if b.Upstream == "" {
		return clr(cDim, "local")
	}
	r := trackRef(b)
	switch {
	case b.Ahead == 0 && b.Behind == 0:
		return clr(cGreen, "= "+r)
	case b.Ahead > 0 && b.Behind == 0:
		return clr(cGreen, fmt.Sprintf("↑%d %s", b.Ahead, r))
	case b.Ahead == 0 && b.Behind > 0:
		return clr(cYellow, fmt.Sprintf("↓%d %s", b.Behind, r))
	default:
		return clr(cBoldRed, fmt.Sprintf("↑%d↓%d %s", b.Ahead, b.Behind, r))
	}
}

// -------------------------------------------------------------------
// Utilities
// -------------------------------------------------------------------

func trunc(s string, max int) string {
	if max <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	if max == 1 {
		return "…"
	}
	return string(runes[:max-1]) + "…"
}

func runeLen(s string) int {
	return len([]rune(s))
}

func getTermWidth() int {
	w, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || w <= 0 {
		return 80
	}
	return w
}
