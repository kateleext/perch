package ui

import "strings"

// Box-drawing diagrams in fenced code blocks are hand-aligned, and hand
// alignment slips. A row that is one column short breaks the right edge of the
// box and is nearly impossible to spot in the source.
//
// This pass finds those rows and pads them for display. It never rewrites the
// file: RawLines keeps the original, so copy and diff still show the truth.
// Rows that cannot be fixed by padding get flagged instead, so a quietly
// corrected diagram is still visibly a broken one.

// BoxNote records what the box-alignment pass concluded about a line.
type BoxNote int

const (
	BoxNoteNone BoxNote = iota
	// BoxNoteAdjusted: the row missed its box's right edge and the run of
	// spaces before its closing border has been grown or shrunk for display.
	BoxNoteAdjusted
	// BoxNoteMisaligned: the row does not meet its box's right edge and
	// whitespace cannot fix it — the content itself overruns, or the closing
	// border is missing. Flagged, not touched.
	BoxNoteMisaligned
)

// maxBoxAdjust caps how far we will move a border. An alignment slip is a
// handful of columns; anything wider is art we don't understand well enough
// to touch.
const maxBoxAdjust = 8

// maxBoxHeight caps how far we scan for a box's closing border.
const maxBoxHeight = 200

var (
	boxTopLeft     = runeSet("┌╭┏╔")
	boxTopRight    = runeSet("┐╮┓╗")
	boxBottomLeft  = runeSet("└╰┗╚")
	boxBottomRight = runeSet("┘╯┛╝")
	boxVertical    = runeSet("│┃║")
	// Border lines carry more than rules: tees and arrowheads mark where a
	// connector meets the box, and they are extremely common in these diagrams.
	boxHorizontal = runeSet("─━═╌╍┬┴┼┯┷╪╤╧▼▲◄►◀▶")
)

func runeSet(s string) map[rune]bool {
	m := make(map[rune]bool, len(s))
	for _, r := range s {
		m[r] = true
	}
	return m
}

// lineRange is a half-open span of line indices.
type lineRange struct{ start, end int }

// boxEdges describes the columns a border line occupies.
type boxEdges struct {
	left  int // visible column of the opening corner
	width int // visible width from column 0 through the closing corner
}

// boxSpan is a located box: its two border lines and the width every row of it
// should reach.
type boxSpan struct {
	top, bottom int
	left        int
	width       int
}

// computeBoxFixes scans fenced code blocks for box diagrams and works out how
// to bring each row onto its box's right edge.
//
// It returns a signed column adjustment per line — positive to insert spaces
// before the closing border, negative to remove them — plus a note for every
// line it changed or could not change. Both maps are nil when there is nothing
// to report.
func computeBoxFixes(lines []string) (map[int]int, map[int]BoxNote) {
	adjust := make(map[int]int)
	notes := make(map[int]BoxNote)

	for _, fence := range fencedRanges(lines) {
		fixBoxesIn(lines, fence, adjust, notes)
	}

	if len(adjust) == 0 {
		adjust = nil
	}
	if len(notes) == 0 {
		notes = nil
	}
	return adjust, notes
}

// fencedRanges returns the body of each fenced code block, mirroring how
// mdRenderer.renderLine tracks fences so the two passes agree on what is code.
func fencedRanges(lines []string) []lineRange {
	var out []lineRange
	inFence := false
	start := 0

	for i, line := range lines {
		if inFence {
			if strings.HasPrefix(strings.TrimSpace(line), "```") {
				out = append(out, lineRange{start, i})
				inFence = false
			}
			continue
		}
		if fenceRegex.MatchString(line) {
			inFence = true
			start = i + 1
		}
	}
	if inFence {
		out = append(out, lineRange{start, len(lines)})
	}
	return out
}

// fixBoxesIn walks one code block, anchoring on each complete box it finds.
func fixBoxesIn(lines []string, fence lineRange, adjust map[int]int, notes map[int]BoxNote) {
	for i := fence.start; i < fence.end; i++ {
		top, ok := parseBoxBorder(lines[i], boxTopLeft, boxTopRight)
		if !ok {
			continue
		}
		bottom, bottomWidth, ok := findBoxBottom(lines, i+1, fence.end, top.left)
		if !ok {
			continue
		}
		box := boxSpan{top: i, bottom: bottom, left: top.left}
		box.width = boxTargetWidth(lines, box, top.width, bottomWidth)
		fixBoxRows(lines, box, adjust, notes)
		// Nothing between here and the closing border can open a box we
		// haven't already considered.
		i = bottom
	}
}

// boxTargetWidth decides how wide the box really is. The two borders are the
// best evidence, but a border can carry the defect itself — an arrow junction
// that nudged a column, say — so we take the most common width across the whole
// box and let the bottom border, which rarely carries junctions, break ties.
func boxTargetWidth(lines []string, box boxSpan, topWidth, bottomWidth int) int {
	counts := make(map[int]int)
	counts[topWidth]++
	counts[bottomWidth]++
	for i := box.top + 1; i < box.bottom; i++ {
		if w, ok := boxRowWidth(lines[i], box.left); ok {
			counts[w]++
		}
	}

	best, bestCount := bottomWidth, 0
	for w, c := range counts {
		if c > bestCount {
			best, bestCount = w, c
		} else if c == bestCount && w == bottomWidth {
			best = w
		}
	}
	return best
}

// boxRowWidth returns the visible width of a line that is a row of the box at
// the given left column, and whether it is one at all.
func boxRowWidth(line string, left int) (int, bool) {
	trimmed := strings.TrimRight(line, " \t")
	if strings.TrimSpace(trimmed) == "" {
		return 0, false
	}
	if countLeadingSpaces(trimmed) != left || !startsWithVertical(trimmed) {
		return 0, false
	}
	return VisibleWidth(trimmed), true
}

// parseBoxBorder reads a horizontal border line: leading whitespace, an opening
// corner, a run of horizontal rules and tees, a closing corner, nothing else.
// Those lines carry no prose, which is what makes them a trustworthy anchor for
// the box's true width.
func parseBoxBorder(line string, open, close map[rune]bool) (boxEdges, bool) {
	trimmed := strings.TrimRight(line, " \t")
	body := []rune(strings.TrimLeft(trimmed, " \t"))
	if len(body) < 3 {
		return boxEdges{}, false
	}
	if !open[body[0]] || !close[body[len(body)-1]] {
		return boxEdges{}, false
	}
	for _, r := range body[1 : len(body)-1] {
		if !boxHorizontal[r] {
			return boxEdges{}, false
		}
	}
	return boxEdges{
		left:  countLeadingSpaces(trimmed),
		width: VisibleWidth(trimmed),
	}, true
}

// findBoxBottom locates the closing border for a box opened at the given left
// column, returning its line index and width.
func findBoxBottom(lines []string, from, end, left int) (int, int, bool) {
	limit := from + maxBoxHeight
	if limit > end {
		limit = end
	}

	for i := from; i < limit; i++ {
		if b, ok := parseBoxBorder(lines[i], boxBottomLeft, boxBottomRight); ok && b.left == left {
			return i, b.width, true
		}
		// A second box opening at our own column means we misread the
		// structure. Better to leave it alone than to guess.
		if b, ok := parseBoxBorder(lines[i], boxTopLeft, boxTopRight); ok && b.left == left {
			return 0, 0, false
		}
	}
	return 0, 0, false
}

// fixBoxRows measures every line of a box against its target width.
func fixBoxRows(lines []string, box boxSpan, adjust map[int]int, notes map[int]BoxNote) {
	// A border that disagrees with the rest of its own box is itself the
	// defect. We only flag it: a border's slack is rule characters, not
	// spaces, and moving them would shift the junctions that connect this box
	// to the ones above and below.
	for _, i := range [2]int{box.top, box.bottom} {
		if VisibleWidth(strings.TrimRight(lines[i], " \t")) != box.width {
			notes[i] = BoxNoteMisaligned
		}
	}

	for i := box.top + 1; i < box.bottom; i++ {
		line := strings.TrimRight(lines[i], " \t")

		// Fully blank rows inside a diagram are common and harmless.
		if strings.TrimSpace(line) == "" {
			continue
		}
		// Only rows that start at the box's own left edge belong to it.
		if countLeadingSpaces(line) != box.left || !startsWithVertical(line) {
			continue
		}

		delta := box.width - VisibleWidth(line)

		switch {
		case !endsWithVertical(line):
			// Nothing to move: the closing border is missing entirely.
			notes[i] = BoxNoteMisaligned
		case delta == 0:
			// Already flush.
		case abs(delta) > maxBoxAdjust:
			notes[i] = BoxNoteMisaligned
		case VisibleWidth(applyBoxAdjust(line, delta)) == box.width:
			adjust[i] = delta
			notes[i] = BoxNoteAdjusted
		default:
			// Narrowing ran out of spaces before it reached the target, so the
			// row's own content is what overruns.
			notes[i] = BoxNoteMisaligned
		}
	}
}

// applyBoxAdjust grows or shrinks the run of spaces immediately before the
// line's closing border. Shrinking only ever consumes spaces, so it cannot
// eat into the row's content; if there isn't enough slack, the line is
// returned unchanged and the caller sees the width didn't land.
func applyBoxAdjust(line string, n int) string {
	if n == 0 {
		return line
	}
	idx := lastVerticalIndex(line)
	if idx < 0 {
		return line
	}
	if n > 0 {
		return line[:idx] + strings.Repeat(" ", n) + line[idx:]
	}

	cut := 0
	for cut < -n && idx-cut-1 >= 0 && line[idx-cut-1] == ' ' {
		cut++
	}
	if cut < -n {
		return line
	}
	return line[:idx-cut] + line[idx:]
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func lastVerticalIndex(s string) int {
	idx := -1
	for i, r := range s {
		if boxVertical[r] {
			idx = i
		}
	}
	return idx
}

func startsWithVertical(line string) bool {
	body := []rune(strings.TrimLeft(line, " \t"))
	return len(body) > 0 && boxVertical[body[0]]
}

func endsWithVertical(line string) bool {
	body := []rune(strings.TrimRight(line, " \t"))
	return len(body) > 0 && boxVertical[body[len(body)-1]]
}
