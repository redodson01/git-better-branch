package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/mattn/go-runewidth"
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
	cBlue     = "\033[34m"
	cCyan     = "\033[36m"
	cBoldGrn  = "\033[1;32m"
	cBoldCyan = "\033[1;36m"
	cBoldRed  = "\033[1;31m"
)

// colorOn is set once in main() before any output. Tests may mutate it
// directly but must not run in parallel (no t.Parallel).
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
	interactive := flag.Bool("i", false, "interactive branch picker")
	flag.BoolVar(interactive, "interactive", false, "interactive branch picker")
	noColor := flag.Bool("no-color", false, "disable colored output")
	showVer := flag.Bool("version", false, "show version")
	flag.BoolVar(showVer, "v", false, "show version")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: git better-branch [flags]

A better git branch viewer.

Flags:
  -a, --all          include remote-tracking branches
  -i, --interactive  interactive branch picker
      --no-color     disable colored output
  -v, --version      show version
`)
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

	tw, th := getTermSize()

	if *interactive {
		if err := runInteractive(branches, tw, th); err != nil {
			fmt.Fprintf(os.Stderr, "gbb: %v\n", err)
			os.Exit(1)
		}
		return
	}

	var local, remote []Branch
	for _, b := range branches {
		if b.IsRemote {
			remote = append(remote, b)
		} else {
			local = append(local, b)
		}
	}

	// Compute column widths globally so local and remote sections align.
	cw := computeWidths(branches, tw)

	var buf bytes.Buffer

	sortBranches(local)
	printBranches(&buf, local, tw, cw)

	if *showAll && len(remote) > 0 {
		fmt.Fprintln(&buf)
		sortBranches(remote)
		printBranches(&buf, remote, tw, cw)
	}

	pageOutput(buf.Bytes(), th)
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

	return parseBranches(raw), nil
}

// parseBranches parses tab-delimited git for-each-ref output into Branch structs.
func parseBranches(raw string) []Branch {
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
				// Atoi errors are intentionally ignored — git's format is well-defined.
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

	return branches
}

// -------------------------------------------------------------------
// Grouping and sorting
// -------------------------------------------------------------------

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

// colWidths holds the computed column widths shared across all sections.
type colWidths struct {
	name   int
	dev    int
	remote int
	hash   int
}

func computeWidths(branches []Branch, tw int) colWidths {
	var cw colWidths
	for _, b := range branches {
		if n := runeLen(b.DisplayName); n > cw.name {
			cw.name = n
		}
		if n := runeLen(devPlain(b)); n > cw.dev {
			cw.dev = n
		}
		if n := runeLen(remotePlain(b)); n > cw.remote {
			cw.remote = n
		}
		if n := runeLen(b.ShortHash); n > cw.hash {
			cw.hash = n
		}
	}

	if cw.remote > 20 {
		cw.remote = 20
	}

	// Layout: indicator(2) + name + gap(1) + dev + gap(1) + remote + gap(1) + hash + gap(1) + tail(>=20)
	nameCap := tw - 2 - 1 - cw.dev - 1 - cw.remote - 1 - cw.hash - 1 - 20
	if nameCap < 20 {
		nameCap = 20
	}
	if nameCap > 50 {
		nameCap = 50
	}
	if cw.name > nameCap {
		cw.name = nameCap
	}

	return cw
}

// printBranches renders branch lines for non-interactive output.
// The interactive mode uses renderLine (interactive.go) instead, which
// packs gap spaces inside each column's color span for clean reverse-video
// selection highlighting. The two paths use intentionally different spacing.
func printBranches(w io.Writer, branches []Branch, tw int, cw colWidths) {
	for _, b := range branches {
		// Indicator.
		var ind string
		switch {
		case b.IsHead:
			ind = clr(cBoldGrn, "*") + " "
		case b.WorktreePath != "":
			ind = clr(cBoldCyan, "+") + " "
		case b.IsRemote:
			ind = clr(cRed, "  ")
		default:
			ind = "  "
		}

		// Name.
		name := trunc(b.DisplayName, cw.name)
		namePad := strings.Repeat(" ", cw.name-runeLen(name))
		switch {
		case b.IsHead:
			name = clr(cBoldGrn, name+namePad)
		case b.WorktreePath != "":
			name = clr(cBoldCyan, name+namePad)
		case b.IsRemote:
			name = clr(cRed, name+namePad)
		default:
			name = name + namePad
		}

		dp := devPlain(b)
		dc := devColored(b)
		dPad := strings.Repeat(" ", cw.dev-runeLen(dp))
		if b.IsRemote && dc == "" {
			dPad = clr(cRed, dPad)
		}

		rp := trunc(remotePlain(b), cw.remote)
		rc := remoteColored(b, cw.remote)
		rPad := strings.Repeat(" ", cw.remote-runeLen(rp))

		hash := b.ShortHash + strings.Repeat(" ", cw.hash-runeLen(b.ShortHash))
		hash = clr(cYellow, hash)

		var wtTag string
		var wtPlainLen int
		if b.WorktreePath != "" {
			wtTag = " " + clr(cCyan, "["+b.WorktreePath+"]")
			wtPlainLen = 3 + runeLen(b.WorktreePath)
		}

		used := 2 + cw.name + 1 + cw.dev + 1 + cw.remote + 1 + cw.hash + 1 + wtPlainLen
		subWidth := tw - used
		if subWidth < 10 {
			subWidth = 10
		}
		subject := trunc(b.Subject, subWidth)

		_, _ = fmt.Fprintf(w, "%s%s %s%s %s%s %s %s%s\n", ind, name, dc, dPad, rc, rPad, hash, subject, wtTag)
	}
}

// pageOutput writes data to stdout, using a pager if it exceeds the terminal height.
// pagerCommand returns the pager command and arguments, following git's
// precedence: GIT_PAGER > core.pager > PAGER, falling back to "less -RFX".
// An empty value for GIT_PAGER or PAGER means "no pager" (returns "", nil).
func pagerCommand() (string, []string) {
	if p, ok := os.LookupEnv("GIT_PAGER"); ok {
		if p == "" {
			return "", nil
		}
		return "sh", []string{"-c", p}
	}
	if out, err := exec.Command("git", "config", "core.pager").Output(); err == nil {
		if p := strings.TrimSpace(string(out)); p != "" {
			return "sh", []string{"-c", p}
		}
	}
	if p, ok := os.LookupEnv("PAGER"); ok {
		if p == "" {
			return "", nil
		}
		return "sh", []string{"-c", p}
	}
	return "less", []string{"-RFX"}
}

func pageOutput(data []byte, th int) {
	if !term.IsTerminal(int(os.Stdout.Fd())) {
		_, _ = os.Stdout.Write(data)
		return
	}

	lines := bytes.Count(data, []byte{'\n'})
	if lines < th {
		_, _ = os.Stdout.Write(data)
		return
	}

	name, args := pagerCommand()
	if name == "" {
		_, _ = os.Stdout.Write(data)
		return
	}
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	pipe, err := cmd.StdinPipe()
	if err != nil {
		_, _ = os.Stdout.Write(data)
		return
	}
	if err := cmd.Start(); err != nil {
		_, _ = os.Stdout.Write(data)
		return
	}
	_, _ = pipe.Write(data)
	_ = pipe.Close()
	_ = cmd.Wait()
}

// -------------------------------------------------------------------
// Deviation + Remote display
// -------------------------------------------------------------------

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

// devPlain returns the deviation indicator as plain text (for width measurement).
func devPlain(b Branch) string {
	if b.IsRemote || b.Upstream == "" {
		return ""
	}
	if b.Gone {
		return "gone"
	}
	switch {
	case b.Ahead == 0 && b.Behind == 0:
		return ""
	case b.Ahead > 0 && b.Behind == 0:
		return fmt.Sprintf("↑%d", b.Ahead)
	case b.Ahead == 0 && b.Behind > 0:
		return fmt.Sprintf("↓%d", b.Behind)
	default:
		return fmt.Sprintf("↑%d↓%d", b.Ahead, b.Behind)
	}
}

// devColored returns the deviation indicator with ANSI colors.
// Used by printBranches (non-interactive). The interactive path uses
// devColorCode instead to build colored gap spans for reverse-video.
func devColored(b Branch) string {
	if b.IsRemote || b.Upstream == "" {
		return ""
	}
	if b.Gone {
		return clr(cBoldRed, "gone")
	}
	switch {
	case b.Ahead == 0 && b.Behind == 0:
		return ""
	case b.Ahead > 0 && b.Behind == 0:
		return clr(cGreen, fmt.Sprintf("↑%d", b.Ahead))
	case b.Ahead == 0 && b.Behind > 0:
		return clr(cYellow, fmt.Sprintf("↓%d", b.Behind))
	default:
		return clr(cBoldRed, fmt.Sprintf("↑%d↓%d", b.Ahead, b.Behind))
	}
}

// remotePlain returns the remote/tracking ref as plain text (for width measurement).
func remotePlain(b Branch) string {
	if b.IsRemote {
		return b.RemoteName
	}
	if b.Upstream == "" {
		return "local"
	}
	return trackRef(b)
}

// remoteColored returns the remote/tracking ref with ANSI colors, truncated to maxWidth.
func remoteColored(b Branch, maxWidth int) string {
	if b.IsRemote {
		return clr(cRed, trunc(b.RemoteName, maxWidth))
	}
	if b.Upstream == "" {
		return clr(cDim, "local")
	}
	return clr(cBlue, trunc(trackRef(b), maxWidth))
}

// -------------------------------------------------------------------
// Utilities
// -------------------------------------------------------------------

// trunc truncates s to fit within max terminal columns, appending "…" if needed.
// Uses display width (not rune count) so CJK/wide characters are handled correctly.
func trunc(s string, max int) string {
	if max <= 0 {
		return ""
	}
	if runewidth.StringWidth(s) <= max {
		return s
	}
	if max == 1 {
		return "…"
	}
	target := max - 1 // reserve 1 column for "…"
	w := 0
	for i, r := range s {
		rw := runewidth.RuneWidth(r)
		if w+rw > target {
			return s[:i] + "…"
		}
		w += rw
	}
	return s
}

// runeLen returns the display width of s in terminal columns.
// CJK/wide characters count as 2 columns.
func runeLen(s string) int {
	return runewidth.StringWidth(s)
}

func getTermSize() (int, int) {
	w, h, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || w <= 0 {
		w = 80
	}
	if err != nil || h <= 0 {
		h = 24
	}
	return w, h
}
