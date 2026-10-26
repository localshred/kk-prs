package render

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/localshred/kk-prs/internal/board"
	"github.com/localshred/koolkit/pkg/ui"
)

const (
	cardInnerWidth = 22
	laneGap        = 2
	laneWidth      = cardInnerWidth + 2
	titleLines     = 3
)

var glyphColors = map[string]func(string) string{
	"✓": ui.Green,
	"✗": ui.Red,
	"↻": ui.Yellow,
}

// Board renders the release board as side by side swim lanes of boxed cards.
// Open and pending lanes always render; release lanes are kept in the given
// order while they fit width, and the rest are summarized on a final line.
// Lane headers share one height so every lane's first card starts on the
// same row.
func Board(b board.Board, width int, showAuthor bool, now time.Time) string {
	if len(b.Lanes) == 0 {
		return "📭 No open or recently merged PRs.\n"
	}
	shown, folded := fitLanes(b.Lanes, width)
	short := sharedOwner(b.Lanes)
	cutRow := slices.ContainsFunc(shown, hasCutLine)
	columns := make([][]string, len(shown))
	height := 0
	for i, l := range shown {
		columns[i] = laneLines(l, cutRow, short, showAuthor, now)
		if len(columns[i]) > height {
			height = len(columns[i])
		}
	}
	blank := strings.Repeat(" ", laneWidth)
	var sb strings.Builder
	for row := 0; row < height; row++ {
		cells := make([]string, len(columns))
		for i, col := range columns {
			cells[i] = blank
			if row < len(col) {
				cells[i] = col[row]
			}
		}
		sb.WriteString(strings.Join(cells, strings.Repeat(" ", laneGap)))
		sb.WriteString("\n")
	}
	if len(folded) > 0 {
		sb.WriteString("\n" + foldSummary(folded) + "\n")
	}
	return sb.String()
}

func boardWidth(lanes int) int {
	if lanes == 0 {
		return 0
	}
	return lanes*laneWidth + (lanes-1)*laneGap
}

func boxLine(content string) string {
	return "│" + content + "│"
}

func cardLines(c board.Card, short bool, showAuthor bool, now time.Time) []string {
	var body []string
	id := fmt.Sprintf("%s#%d", repoName(c.PR.Repo, short), c.PR.Number)
	body = append(body, fit(id, func(s string) string { return ui.Hyperlink(c.PR.URL, ui.Cyan(s)) }, cardInnerWidth))
	if c.WorkItemID != "" {
		body = append(body, fit(c.WorkItemID, func(s string) string { return ui.Hyperlink(c.WorkItemURL, s) }, cardInnerWidth))
	}
	for _, l := range wrapTitle(c.PR.Title, cardInnerWidth, titleLines) {
		body = append(body, fit(l, nil, cardInnerWidth))
	}
	body = append(body, stateLine(c, now))
	if c.Reason != "" {
		body = append(body, fit(c.Reason, ui.Dim, cardInnerWidth))
	}
	if !c.InReleaseAt.IsZero() {
		body = append(body, fit("in release "+ui.Humanize(now.Sub(c.InReleaseAt)), nil, cardInnerWidth))
	}
	if c.Via != "" {
		body = append(body, fit(viaText(c.Via), nil, cardInnerWidth))
	}
	if showAuthor {
		body = append(body, fit("@"+c.PR.Author, ui.Yellow, cardInnerWidth))
	}
	border := strings.Repeat("─", cardInnerWidth)
	lines := []string{"┌" + border + "┐"}
	for _, l := range body {
		lines = append(lines, boxLine(l))
	}
	return append(lines, "└"+border+"┘")
}

func fit(text string, style func(string) string, width int) string {
	text = truncate(text, width)
	pad := width - len([]rune(text))
	if style != nil {
		text = style(text)
	}
	return text + strings.Repeat(" ", pad)
}

func fitLanes(lanes []board.Lane, width int) ([]board.Lane, []board.Lane) {
	var shown, releases []board.Lane
	for _, l := range lanes {
		if l.Kind == board.LaneRelease {
			releases = append(releases, l)
		} else {
			shown = append(shown, l)
		}
	}
	for i, l := range releases {
		if boardWidth(len(shown)+1) > width {
			return shown, releases[i:]
		}
		shown = append(shown, l)
	}
	return shown, nil
}

func foldSummary(folded []board.Lane) string {
	parts := make([]string, len(folded))
	for i, l := range folded {
		parts[i] = fmt.Sprintf("%s (%d)", l.Name, len(l.Cards))
	}
	noun := "releases"
	if len(folded) == 1 {
		noun = "release"
	}
	return fmt.Sprintf("+%d older %s: %s", len(folded), noun, strings.Join(parts, ", "))
}

func hasCutLine(l board.Lane) bool {
	return l.Kind == board.LaneRelease && !l.CutAt.IsZero()
}

func laneLines(l board.Lane, cutRow bool, short bool, showAuthor bool, now time.Time) []string {
	lines := []string{fit(fmt.Sprintf("%s (%d)", l.Name, len(l.Cards)), nil, laneWidth)}
	switch {
	case hasCutLine(l):
		cut := fmt.Sprintf("cut ~%s (%s)", l.CutAt.Format("Jan 2"), ui.Humanize(now.Sub(l.CutAt)))
		lines = append(lines, fit(cut, ui.Dim, laneWidth))
	case cutRow:
		lines = append(lines, strings.Repeat(" ", laneWidth))
	}
	for _, c := range l.Cards {
		lines = append(lines, cardLines(c, short, showAuthor, now)...)
	}
	return lines
}

func owner(repo string) string {
	o, _, _ := strings.Cut(repo, "/")
	return o
}

func repoName(repo string, short bool) string {
	if !short {
		return repo
	}
	_, name, found := strings.Cut(repo, "/")
	if !found {
		return repo
	}
	return name
}

func sharedOwner(lanes []board.Lane) bool {
	first := ""
	for _, l := range lanes {
		for _, c := range l.Cards {
			o := owner(c.PR.Repo)
			if first == "" {
				first = o
			}
			if o != first {
				return false
			}
		}
	}
	return true
}

func stateLine(c board.Card, now time.Time) string {
	if c.PR.State == "MERGED" {
		return fit("merged "+ui.Humanize(now.Sub(c.PR.MergedAt)), ui.Dim, cardInnerWidth)
	}
	age := "opened " + ui.Humanize(now.Sub(c.PR.CreatedAt))
	if color, ok := glyphColors[c.CheckGlyph]; ok {
		return color(c.CheckGlyph) + " " + fit(age, ui.Dim, cardInnerWidth-2)
	}
	return fit(age, ui.Dim, cardInnerWidth)
}

func truncate(s string, width int) string {
	r := []rune(s)
	if len(r) <= width {
		return s
	}
	return string(r[:width-1]) + "…"
}

func viaText(via string) string {
	if strings.HasPrefix(via, "#") {
		return "↳ via hotfix " + via
	}
	return "↳ via " + via
}

func wrapTitle(title string, width, maxLines int) []string {
	var lines []string
	var line []rune
	for _, word := range strings.Fields(title) {
		r := []rune(word)
		if len(line) > 0 && len(line)+1+len(r) <= width {
			line = append(append(line, ' '), r...)
			continue
		}
		if len(line) > 0 {
			lines = append(lines, string(line))
		}
		for len(r) > width {
			lines = append(lines, string(r[:width]))
			r = r[width:]
		}
		line = r
	}
	if len(line) > 0 {
		lines = append(lines, string(line))
	}
	if len(lines) == 0 {
		return []string{""}
	}
	if len(lines) > maxLines {
		rest := strings.Join(lines[maxLines-1:], " ")
		lines = append(lines[:maxLines-1], truncate(rest, width))
	}
	return lines
}
