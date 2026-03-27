package main

import (
	"fmt"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

const cReverse = "\033[7m"

// listItem represents one row in the interactive display.
type listItem struct {
	branch *Branch // non-nil for selectable branch rows
	header string  // section header text (non-selectable)
	blank  bool    // blank separator line (non-selectable)
}

type tuiModel struct {
	items  []listItem
	selIdx []int // indices into items that are selectable (branches only)
	cursor int   // index into selIdx
	offset int   // viewport scroll offset (items index)
	tw, th int
	cw     colWidths
	chosen *Branch // set on Enter, nil on quit
}

func runInteractive(branches []Branch, tw, th int) error {
	var local, remote []Branch
	for _, b := range branches {
		if b.IsRemote {
			remote = append(remote, b)
		} else {
			local = append(local, b)
		}
	}

	cw := computeWidths(branches, tw)

	var items []listItem
	var selIdx []int

	sortBranches(local)
	for i := range local {
		selIdx = append(selIdx, len(items))
		items = append(items, listItem{branch: &local[i]})
	}

	if len(remote) > 0 {
		groups := groupByRemote(remote)
		for gi := range groups {
			g := &groups[gi]
			items = append(items, listItem{blank: true})
			items = append(items, listItem{header: fmt.Sprintf("remote/%s:", g.name)})
			sortBranches(g.branches)
			for i := range g.branches {
				selIdx = append(selIdx, len(items))
				items = append(items, listItem{branch: &g.branches[i]})
			}
		}
	}

	if len(selIdx) == 0 {
		return nil
	}

	m := tuiModel{
		items:  items,
		selIdx: selIdx,
		tw:     tw,
		th:     th,
		cw:     cw,
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	final, err := p.Run()
	if err != nil {
		return err
	}

	fm := final.(tuiModel)
	if fm.chosen != nil {
		return checkoutBranch(fm.chosen)
	}
	return nil
}

func checkoutBranch(b *Branch) error {
	name := b.Name
	if b.IsRemote {
		name = b.DisplayName
	}

	cmd := exec.Command("git", "checkout", name)
	out, err := cmd.CombinedOutput()
	output := strings.TrimSpace(string(out))
	if output != "" {
		fmt.Println(output)
	}
	if err != nil {
		return fmt.Errorf("checkout failed: %w", err)
	}
	return nil
}

// --- bubbletea Model interface ---

func (m tuiModel) Init() tea.Cmd {
	return nil
}

func (m tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.tw = msg.Width
		m.th = msg.Height

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
				m.ensureVisible()
			}
		case "down", "j":
			if m.cursor < len(m.selIdx)-1 {
				m.cursor++
				m.ensureVisible()
			}
		case "enter":
			m.chosen = m.items[m.selIdx[m.cursor]].branch
			return m, tea.Quit
		}
	}

	return m, nil
}

func (m tuiModel) View() string {
	viewH := m.viewHeight()
	selectedItemIdx := m.selIdx[m.cursor]

	end := m.offset + viewH
	if end > len(m.items) {
		end = len(m.items)
	}

	var lines []string
	for i := m.offset; i < end; i++ {
		item := m.items[i]
		var line string

		switch {
		case item.blank:
			line = ""
		case item.header != "":
			line = clr(cBold+cBlue, item.header)
		default:
			line = renderLine(item.branch, m.cw, m.tw)
		}

		if i == selectedItemIdx {
			// Pad to full width so reverse video spans the entire row.
			visible := runeLen(stripAnsi(line))
			if visible < m.tw {
				line += strings.Repeat(" ", m.tw-visible)
			}
			// Apply reverse video, re-enabling after each color reset.
			line = cReverse + strings.ReplaceAll(line, cReset, cReset+cReverse) + cReset
		}

		lines = append(lines, line)
	}

	for len(lines) < viewH {
		lines = append(lines, "")
	}

	lines = append(lines, "")
	lines = append(lines, clr(cDim, "  ↑/↓ navigate  enter checkout  q quit"))

	return strings.Join(lines, "\n")
}

// --- viewport helpers ---

func (m *tuiModel) ensureVisible() {
	viewH := m.viewHeight()
	itemIdx := m.selIdx[m.cursor]

	if itemIdx < m.offset {
		m.offset = itemIdx
	}
	if itemIdx >= m.offset+viewH {
		m.offset = itemIdx - viewH + 1
	}
}

func (m tuiModel) viewHeight() int {
	h := m.th - 2 // room for blank line + status bar
	if h < 1 {
		return 1
	}
	return h
}

// --- line rendering (with colored gaps for clean reverse-video) ---

// renderLine produces a branch line where each column's trailing gap space
// is inside that column's color span. This ensures reverse video shows a
// continuous colored background instead of gray patches between columns.
func renderLine(b *Branch, cw colWidths, tw int) string {
	if b.IsRemote {
		return renderRemoteLine(b, cw, tw)
	}
	return renderLocalLine(b, cw, tw)
}

func renderLocalLine(b *Branch, cw colWidths, tw int) string {
	// Each column has a leading and trailing space in its own color,
	// producing 2 colored spaces between adjacent columns.
	//
	// Layout: ind(2) | name+trail(cw.name+1) | lead+dev+trail(1+cw.dev+1) |
	//         lead+remote+trail(1+cw.remote+1) | lead+hash+trail(1+cw.hash+1) |
	//         lead+subject(1+...)

	// Indicator: trailing space inside color span.
	var ind string
	switch {
	case b.IsHead:
		ind = clr(cBoldGrn, "* ")
	case b.WorktreePath != "":
		ind = clr(cBoldCyan, "+ ")
	default:
		ind = "  "
	}

	// Name: content + pad + trailing space, in name color.
	nameText := trunc(b.DisplayName, cw.name)
	nameTrail := strings.Repeat(" ", cw.name-runeLen(nameText)+1)
	var name string
	switch {
	case b.IsHead:
		name = clr(cBoldGrn, nameText+nameTrail)
	case b.WorktreePath != "":
		name = clr(cBoldCyan, nameText+nameTrail)
	default:
		name = nameText + nameTrail
	}

	// Deviation: leading space + content + pad + trailing space, in dev color.
	dp := devPlain(*b)
	devBody := " " + dp + strings.Repeat(" ", cw.dev-runeLen(dp)+1)
	var dev string
	if dc := devColorCode(*b); dc != "" {
		dev = clr(dc, devBody)
	} else {
		dev = devBody
	}

	// Remote: leading space + content + pad + trailing space, in remote color.
	rp := trunc(remotePlain(*b), cw.remote)
	remBody := " " + rp + strings.Repeat(" ", cw.remote-runeLen(rp)+1)
	var rem string
	if b.Upstream == "" {
		rem = clr(cDim, remBody)
	} else {
		rem = clr(cBlue, remBody)
	}

	// Hash: leading space + content + pad + trailing space, in yellow.
	hashBody := " " + b.ShortHash + strings.Repeat(" ", cw.hash-runeLen(b.ShortHash)+1)
	hash := clr(cYellow, hashBody)

	// Subject: leading space + content + optional worktree tag.
	var wtTag string
	var wtPlainLen int
	if b.WorktreePath != "" {
		wtTag = " " + clr(cCyan, "["+b.WorktreePath+"]")
		wtPlainLen = 3 + runeLen(b.WorktreePath)
	}

	used := 2 + (cw.name + 1) + (1 + cw.dev + 1) + (1 + cw.remote + 1) + (1 + cw.hash + 1) + 1 + wtPlainLen
	subWidth := tw - used
	if subWidth < 10 {
		subWidth = 10
	}
	subject := " " + trunc(b.Subject, subWidth) + wtTag

	return ind + name + dev + rem + hash + subject
}

func renderRemoteLine(b *Branch, cw colWidths, tw int) string {
	ind := "  "

	// Remote name absorbs the name+dev+remote columns (including their extra gaps).
	extName := cw.name + 1 + (1 + cw.dev + 1) + (1 + cw.remote)
	nameText := trunc(b.DisplayName, extName)
	nameTrail := strings.Repeat(" ", extName-runeLen(nameText)+1)
	name := nameText + nameTrail

	// Hash: leading space + content + pad + trailing space, in yellow.
	hashBody := " " + b.ShortHash + strings.Repeat(" ", cw.hash-runeLen(b.ShortHash)+1)
	hash := clr(cYellow, hashBody)

	// Subject: leading space + content.
	used := 2 + (extName + 1) + (1 + cw.hash + 1) + 1
	subWidth := tw - used
	if subWidth < 10 {
		subWidth = 10
	}
	subject := " " + trunc(b.Subject, subWidth)

	return ind + name + hash + subject
}

// devColorCode returns just the ANSI color code for the deviation state.
func devColorCode(b Branch) string {
	if b.IsRemote || b.Upstream == "" {
		return ""
	}
	if b.Gone {
		return cBoldRed
	}
	switch {
	case b.Ahead == 0 && b.Behind == 0:
		return "" // synced — no color
	case b.Ahead > 0 && b.Behind == 0:
		return cGreen
	case b.Ahead == 0 && b.Behind > 0:
		return cYellow
	default:
		return cBoldRed
	}
}

// stripAnsi removes ANSI SGR escape sequences from a string.
func stripAnsi(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	inEsc := false
	for _, r := range s {
		if inEsc {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inEsc = false
			}
			continue
		}
		if r == '\033' {
			inEsc = true
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}
