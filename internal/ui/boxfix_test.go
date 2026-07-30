package ui

import (
	"strings"
	"testing"
)

// Lifted from a real ARCHITECTURE.md. Three defects, all invisible in source:
// THE STEWARD's closing border is missing and its text overruns by 6 columns;
// the "runs" and "decides nothing" rows are each one column short.
const boxFixture = "```\n" +
	"   ┌────────────────────────────────────────────────────────────────┐\n" +
	"   │  THE SOURCE                                                    │\n" +
	"   │  CalAccess rebuilds dbwebexport.zip nightly · 1.57 GB          │\n" +
	"   └────────────────────────────┬───────────────────────────────────┘\n" +
	"                                │  pull, when the ETag changes\n" +
	"                                ▼\n" +
	"   ┌────────────────────────────────────────────────────────────────┐\n" +
	"   │  THE STEWARD                        one per state, ours or a partner's\n" +
	"   │                                                                │\n" +
	"   └────────────────────────────┬───────────────────────────────────┘\n" +
	"                                ▼\n" +
	"   ┌────────────────────────────────────────────────────────────────┐\n" +
	"   │  THE COMMONS                        one, shared, always on     │\n" +
	"   │                                                                │\n" +
	"   │    runs             what each steward did, and when           │\n" +
	"   │                                                                │\n" +
	"   │  decides nothing · admits or refuses                          │\n" +
	"   └───────────┬────────────────────────────────────┬───────────────┘\n" +
	"               │                                    │\n" +
	"               ▼                                    ▼\n" +
	"```"

func TestComputeBoxFixesRealDiagram(t *testing.T) {
	lines := strings.Split(boxFixture, "\n")
	pad, notes := computeBoxFixes(lines)

	wantAdjust := map[int]int{15: 1, 17: 1}
	if len(pad) != len(wantAdjust) {
		t.Fatalf("pad = %v, want %v", pad, wantAdjust)
	}
	for idx, n := range wantAdjust {
		if pad[idx] != n {
			t.Errorf("pad[%d] = %d, want %d (line %q)", idx, pad[idx], n, lines[idx])
		}
	}

	if notes[8] != BoxNoteMisaligned {
		t.Errorf("line 8 note = %v, want BoxNoteMisaligned (overruns, no closing border)", notes[8])
	}
	for _, idx := range []int{15, 17} {
		if notes[idx] != BoxNoteAdjusted {
			t.Errorf("line %d note = %v, want BoxNoteAdjusted", idx, notes[idx])
		}
	}
	if len(notes) != 3 {
		t.Errorf("notes = %v, want exactly 3 entries", notes)
	}
}

func TestComputeBoxFixesProducesFlushEdges(t *testing.T) {
	lines := strings.Split(boxFixture, "\n")
	pad, _ := computeBoxFixes(lines)

	// The last box: every row should reach column 69 once padded.
	for i := 12; i <= 18; i++ {
		got := VisibleWidth(applyBoxAdjust(lines[i], pad[i]))
		if got != 69 {
			t.Errorf("line %d width = %d, want 69: %q", i, got, lines[i])
		}
	}
}

func TestComputeBoxFixesLeavesConnectorsAlone(t *testing.T) {
	lines := strings.Split(boxFixture, "\n")
	pad, notes := computeBoxFixes(lines)

	// Lines 19 and 20 sit between boxes and happen to carry two verticals.
	// They are not box rows and must not be stretched to the box width.
	for _, idx := range []int{19, 20} {
		if pad[idx] != 0 || notes[idx] != BoxNoteNone {
			t.Errorf("connector line %d was touched: pad=%d note=%v", idx, pad[idx], notes[idx])
		}
	}
}

func TestComputeBoxFixesIgnoresProse(t *testing.T) {
	// Box characters outside a fence belong to prose or rendered tables and
	// are not ours to touch.
	lines := []string{
		"   ┌──────────┐",
		"   │  hello   │",
		"   │  short  │",
		"   └──────────┘",
	}
	pad, notes := computeBoxFixes(lines)
	if pad != nil || notes != nil {
		t.Errorf("unfenced content was modified: pad=%v notes=%v", pad, notes)
	}
}

func TestComputeBoxFixesOutvotesABrokenBorder(t *testing.T) {
	// The top border says 15 wide; the bottom border and both rows say 14.
	// The border is the outlier, so it gets flagged and the rows stand.
	lines := []string{
		"```",
		"┌─────────────┐",
		"│  a         │",
		"│  b         │",
		"└────────────┘",
		"```",
	}
	pad, notes := computeBoxFixes(lines)
	if pad != nil {
		t.Errorf("pad = %v, want none — the rows already agree", pad)
	}
	if notes[1] != BoxNoteMisaligned {
		t.Errorf("top border note = %v, want BoxNoteMisaligned", notes[1])
	}
	if len(notes) != 1 {
		t.Errorf("notes = %v, want only the top border flagged", notes)
	}
}

func TestComputeBoxFixesReadsBordersWithJunctions(t *testing.T) {
	// Border lines routinely carry an arrowhead or tee where a connector
	// meets the box. Those still have to parse as borders.
	lines := []string{
		"```",
		"  ╔════════════▼════════════╗",
		"  ║  alpha                  ║",
		"  ║  beta                  ║",
		"  ╚═══════╤═════════════════╝",
		"```",
	}
	pad, notes := computeBoxFixes(lines)
	if pad[3] != 1 || notes[3] != BoxNoteAdjusted {
		t.Fatalf("row below an arrowed border not padded: pad=%v notes=%v", pad, notes)
	}
	for i := 1; i <= 4; i++ {
		if got := VisibleWidth(applyBoxAdjust(lines[i], pad[i])); got != 29 {
			t.Errorf("line %d width = %d, want 29", i, got)
		}
	}
}

func TestComputeBoxFixesRefusesWideGaps(t *testing.T) {
	// A row 30 columns short is not an alignment slip; leave it flagged only.
	lines := []string{
		"```",
		"┌──────────────────────────────────────┐",
		"│  a                                  │",
		"│  b │",
		"└──────────────────────────────────────┘",
		"```",
	}
	pad, notes := computeBoxFixes(lines)
	if _, ok := pad[3]; ok {
		t.Errorf("padded a %d-column gap, want none", 40-VisibleWidth(lines[3]))
	}
	if notes[3] != BoxNoteMisaligned {
		t.Errorf("line 3 note = %v, want BoxNoteMisaligned", notes[3])
	}
	if pad[2] != 1 {
		t.Errorf("pad[2] = %d, want 1", pad[2])
	}
}

func TestComputeBoxFixesHandlesBlankAndRoundedBoxes(t *testing.T) {
	lines := []string{
		"```",
		"╭────────────╮",
		"│  alpha     │",
		"",
		"│  beta     │",
		"╰────────────╯",
		"```",
	}
	pad, notes := computeBoxFixes(lines)
	if pad[4] != 1 || notes[4] != BoxNoteAdjusted {
		t.Errorf("rounded box row not padded: pad=%d note=%v", pad[4], notes[4])
	}
	if _, ok := pad[3]; ok {
		t.Error("blank row inside box was padded")
	}
	if notes[3] != BoxNoteNone {
		t.Errorf("blank row inside box was flagged: %v", notes[3])
	}
}

func TestApplyBoxAdjustMovesTheClosingBorder(t *testing.T) {
	cases := []struct {
		name string
		line string
		n    int
		want string
	}{
		{"widen", "   │  runs   │", 3, "   │  runs      │"},
		{"narrow", "   │  runs   │", -2, "   │  runs │"},
		{"no border", "no borders here", 3, "no borders here"},
		{"zero", "   │  runs   │", 0, "   │  runs   │"},
		// Only spaces may be consumed. There are two here, not four, so the
		// line comes back untouched rather than losing a character of content.
		{"not enough slack", "   │  runs  │", -4, "   │  runs  │"},
	}
	for _, tc := range cases {
		if got := applyBoxAdjust(tc.line, tc.n); got != tc.want {
			t.Errorf("%s: applyBoxAdjust(%q, %d) = %q, want %q", tc.name, tc.line, tc.n, got, tc.want)
		}
	}
}

func TestComputeBoxFixesNarrowsOverwideRows(t *testing.T) {
	// Two rows carry one space too many. The box and its borders agree on 14,
	// so those rows get pulled back in rather than merely flagged.
	lines := []string{
		"```",
		"┌────────────┐",
		"│  alpha     │",
		"│  beta       │",
		"│  gamma      │",
		"└────────────┘",
		"```",
	}
	adjust, notes := computeBoxFixes(lines)

	for _, idx := range []int{3, 4} {
		if adjust[idx] != -1 || notes[idx] != BoxNoteAdjusted {
			t.Errorf("line %d: adjust=%d note=%v, want -1 / BoxNoteAdjusted", idx, adjust[idx], notes[idx])
		}
	}
	for i := 1; i <= 5; i++ {
		if got := VisibleWidth(applyBoxAdjust(lines[i], adjust[i])); got != 14 {
			t.Errorf("line %d width = %d, want 14", i, got)
		}
	}
	if len(notes) != 2 {
		t.Errorf("notes = %v, want exactly 2 entries", notes)
	}
}

func TestComputeBoxFixesFlagsRowsWithNoSlackToGive(t *testing.T) {
	// This row is two columns too wide but has only one space before its
	// border. Narrowing would eat content, so we flag instead.
	lines := []string{
		"```",
		"┌────────────┐",
		"│  alpha     │",
		"│  beta·······x │",
		"└────────────┘",
		"```",
	}
	adjust, notes := computeBoxFixes(lines)
	if _, ok := adjust[3]; ok {
		t.Errorf("adjust[3] = %d, want no adjustment", adjust[3])
	}
	if notes[3] != BoxNoteMisaligned {
		t.Errorf("line 3 note = %v, want BoxNoteMisaligned", notes[3])
	}
}

func TestHighlightMarkdownLinesLeavesInputUntouched(t *testing.T) {
	lines := strings.Split(boxFixture, "\n")
	before := append([]string(nil), lines...)

	_, notes := highlightMarkdownLines(lines, "ARCHITECTURE.md")

	for i := range lines {
		if lines[i] != before[i] {
			t.Fatalf("input line %d mutated: %q -> %q", i, before[i], lines[i])
		}
	}
	if len(notes) != 3 {
		t.Errorf("notes = %v, want 3 entries", notes)
	}
}
