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
	cursor int   // index into the active selection list
	offset int   // viewport scroll offset
	tw, th int
	cw     colWidths
	chosen *Branch // set on Enter, nil on quit

	// Search state.
	searching   bool
	query       string
	filteredIdx []int // indices into items matching the query
	savedCursor int   // cursor before entering search
	savedOffset int   // offset before entering search
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
		if m.searching {
			return m.updateSearch(msg)
		}
		return m.updateNormal(msg)
	}
	return m, nil
}

func (m tuiModel) updateNormal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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
	case "/":
		m.savedCursor = m.cursor
		m.savedOffset = m.offset
		m.searching = true
		m.query = ""
		m.filteredIdx = nil
	}
	return m, nil
}

func (m tuiModel) updateSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.searching = false
		m.query = ""
		m.filteredIdx = nil
		m.cursor = m.savedCursor
		m.offset = m.savedOffset
	case "ctrl+c":
		return m, tea.Quit
	case "enter":
		vis := m.visibleSel()
		if len(vis) > 0 && m.cursor < len(vis) {
			m.chosen = m.items[vis[m.cursor]].branch
			return m, tea.Quit
		}
	case "up":
		if m.cursor > 0 {
			m.cursor--
			m.ensureVisible()
		}
	case "down":
		vis := m.visibleSel()
		if len(vis) > 0 && m.cursor < len(vis)-1 {
			m.cursor++
			m.ensureVisible()
		}
	case "backspace", "ctrl+h":
		if len(m.query) > 0 {
			runes := []rune(m.query)
			m.query = string(runes[:len(runes)-1])
			m.applyFilter()
		}
	default:
		if len(msg.Runes) > 0 {
			m.query += string(msg.Runes)
			m.applyFilter()
		}
	}
	return m, nil
}

// visibleSel returns the active list of selectable item indices.
func (m tuiModel) visibleSel() []int {
	if m.searching && len(m.query) > 0 {
		return m.filteredIdx
	}
	return m.selIdx
}

func (m *tuiModel) applyFilter() {
	m.filteredIdx = nil
	if m.query == "" {
		m.cursor = 0
		m.offset = 0
		return
	}
	for _, idx := range m.selIdx {
		b := m.items[idx].branch
		if fuzzyMatch(m.query, searchTarget(b)) {
			m.filteredIdx = append(m.filteredIdx, idx)
		}
	}
	m.cursor = 0
	m.offset = 0
}

// --- view ---

func (m tuiModel) View() string {
	viewH := m.viewHeight()
	var lines []string

	if m.searching && len(m.query) > 0 {
		lines = m.renderFilteredView(viewH)
	} else {
		lines = m.renderNormalView(viewH)
	}

	for len(lines) < viewH {
		lines = append(lines, "")
	}

	// Status bar / search prompt.
	lines = append(lines, "")
	if m.searching {
		lines = append(lines, clr(cBold, "/") + m.query + clr(cDim, "▏"))
	} else {
		lines = append(lines, clr(cDim, "  ↑/↓ navigate  enter checkout  / search  q quit"))
	}

	return strings.Join(lines, "\n")
}

func (m tuiModel) renderNormalView(viewH int) []string {
	selectedItemIdx := -1
	if m.cursor < len(m.selIdx) {
		selectedItemIdx = m.selIdx[m.cursor]
	}

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
			line = applySelection(line, m.tw)
		}

		lines = append(lines, line)
	}
	return lines
}

func (m tuiModel) renderFilteredView(viewH int) []string {
	vis := m.filteredIdx
	end := m.offset + viewH
	if end > len(vis) {
		end = len(vis)
	}

	var lines []string
	for i := m.offset; i < end; i++ {
		b := m.items[vis[i]].branch
		line := renderLine(b, m.cw, m.tw)

		if i == m.cursor {
			line = applySelection(line, m.tw)
		}

		lines = append(lines, line)
	}
	return lines
}

func applySelection(line string, tw int) string {
	visible := runeLen(stripAnsi(line))
	if visible < tw {
		line += strings.Repeat(" ", tw-visible)
	}
	if !colorOn {
		return line
	}
	return cReverse + strings.ReplaceAll(line, cReset, cReset+cReverse) + cReset
}

// --- viewport helpers ---

func (m *tuiModel) ensureVisible() {
	viewH := m.viewHeight()

	if m.searching && len(m.query) > 0 {
		// cursor and offset are into filteredIdx.
		if m.cursor < m.offset {
			m.offset = m.cursor
		}
		if m.cursor >= m.offset+viewH {
			m.offset = m.cursor - viewH + 1
		}
	} else {
		// Normal mode: offset is into items, cursor is into selIdx.
		itemIdx := m.selIdx[m.cursor]
		if itemIdx < m.offset {
			m.offset = itemIdx
		}
		if itemIdx >= m.offset+viewH {
			m.offset = itemIdx - viewH + 1
		}
	}
}

func (m tuiModel) viewHeight() int {
	h := m.th - 2 // room for blank line + status bar
	if h < 1 {
		return 1
	}
	return h
}

// --- fuzzy search ---

func fuzzyMatch(query, target string) bool {
	q := []rune(strings.ToLower(query))
	t := strings.ToLower(target)
	qi := 0
	for _, r := range t {
		if qi < len(q) && r == q[qi] {
			qi++
		}
	}
	return qi == len(q)
}

func searchTarget(b *Branch) string {
	s := b.DisplayName
	if b.IsRemote {
		s += " " + b.RemoteName
	} else if b.Upstream != "" {
		s += " " + trackRef(*b)
	}
	return s
}

// --- line rendering (with colored gaps for clean reverse-video) ---

// renderLine produces a branch line where each column's gap spaces
// are inside that column's color span. This ensures reverse video shows a
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
		return ""
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
