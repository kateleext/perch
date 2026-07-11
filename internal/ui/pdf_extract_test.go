package ui

import (
	"strings"
	"testing"
)

// TestIsGarbledText_NormalProse ensures ordinary English prose is never
// flagged as garbled, across a few different shapes of content (sentences,
// short table-like fragments, numbers/punctuation-heavy lines).
func TestIsGarbledText_NormalProse(t *testing.T) {
	cases := []string{
		"This is a normal test document for the perch PDF preview. " +
			"It contains ordinary English sentences with common words like " +
			"the, and, of, to, in, is, for, with, you, that, and this. " +
			"We want to confirm that the extraction pipeline renders this " +
			"text correctly without any glyph soup or garbled output.",
		"One Philosophy, Three Depths\nWe price by deliverable, never by the hour. " +
			"We build with you, not for you, and every engagement is designed to end: " +
			"success means your team runs all of it without us.",
	}
	for _, s := range cases {
		if isGarbledText(s) {
			t.Errorf("normal prose incorrectly flagged as garbled: %q", s)
		}
	}
}

// TestIsGarbledText_ShortSamplesGetBenefitOfDoubt checks that short
// fragments (labels, headers, table cells) aren't false-positived just
// because they're too short to judge statistically.
func TestIsGarbledText_ShortSamplesGetBenefitOfDoubt(t *testing.T) {
	cases := []string{"", "TAKUMA", "$10-20K", "Page 3", "Q1 2026 Revenue"}
	for _, s := range cases {
		if isGarbledText(s) {
			t.Errorf("short sample incorrectly flagged as garbled: %q", s)
		}
	}
}

// TestIsGarbledText_GlyphSoup reproduces the two failure shapes seen in the
// wild from github.com/ledongthuc/pdf mis-decoding a Chrome/Skia PDF's font
// encoding: (1) every letter mapped to a different-but-still-printable-ASCII
// character, and (2) letters mapped to unrelated accented/symbol glyphs.
func TestIsGarbledText_GlyphSoup(t *testing.T) {
	cases := []string{
		// Shape 1: printable-ASCII substitution cipher (real observed output).
		"7IhkQEIDwGI[QtIk<D[I^ItIkDwnPIP`ok7IDoQ[GuQnPw`o^`nN`kw`o<^GItIkw " +
			"I^O<OI]I^nQlGIlQO^IGn`I^GloEEIll]I<^lw`oknI<]ko^l<[[`NQnuQnP`onol " +
			"oh<^<kEPQnIEnokI]<h<hkQ`kQnQzIGh[<^<^G<nI<]`hIk<nQ^O`^Qnl`u^uQnP`k",
		// Shape 2: wrong Unicode glyphs entirely (accented/symbol soup).
		"GºR £³ÁÓÁÌ ì¦_ ÏÌÙ Ó",
	}
	for _, s := range cases {
		if !isGarbledText(s) {
			t.Errorf("glyph soup not detected as garbled: %q", s)
		}
	}
}

// TestIsGarbledText_NonEnglishProseNotGarbled ensures legitimate non-English
// Latin-script prose (accented characters, typographic quotes) is not
// mistaken for glyph soup just because it doesn't match English stop words.
func TestIsGarbledText_NonEnglishProseNotGarbled(t *testing.T) {
	cases := []string{
		"Le présent contrat est conclu entre les parties soussignées, ci-après " +
			"dénommées « le Prestataire » et « le Client ». Les parties conviennent " +
			"que le Prestataire fournira les services décrits à l'Annexe A selon le " +
			"calendrier convenu. Toute modification du présent accord devra être " +
			"établie par écrit et signée par les deux parties. En cas de résiliation " +
			"anticipée, le Client sera tenu de régler les prestations déjà exécutées " +
			"à la date de résiliation.",
		"Der vorliegende Vertrag wird zwischen den unterzeichnenden Parteien " +
			"geschlossen, im Folgenden als „Auftragnehmer“ und „Kunde“ bezeichnet. " +
			"Die Parteien vereinbaren, dass der Auftragnehmer die in Anlage A " +
			"beschriebenen Leistungen gemäß dem vereinbarten Zeitplan erbringt. Jede " +
			"Änderung dieser Vereinbarung bedarf der Schriftform und der " +
			"Unterschrift beider Parteien. Im Falle einer vorzeitigen Kündigung ist " +
			"der Kunde verpflichtet, die bereits erbrachten Leistungen bis zum " +
			"Kündigungsdatum zu vergüten.",
	}
	for _, s := range cases {
		if isGarbledText(s) {
			t.Errorf("legitimate non-English prose incorrectly flagged as garbled: %q", s)
		}
	}
}

// TestIsGarbledText_MathNotationNotGarbled ensures symbol-dense technical
// text (standard Unicode math notation) isn't mistaken for glyph soup just
// because it's punctuation/symbol heavy.
func TestIsGarbledText_MathNotationNotGarbled(t *testing.T) {
	s := "For all x ∈ ℝ, ∃ y such that ∂f/∂x = ∫ λ dΣ over the domain, where " +
		"x ∈ A ∩ B implies x ∈ A ∪ B, and ‖x‖ ≤ ∞ with error bounded by ε → 0 as " +
		"n → ∞, so ∇f(x) ≈ 0 whenever λ ≠ 0 and x ∉ ∅."
	if isGarbledText(s) {
		t.Errorf("legitimate math notation incorrectly flagged as garbled: %q", s)
	}
}

// TestExtractPdfStructured_NormalPDF is an end-to-end sanity check against a
// small synthetic PDF fixture (testdata/sample_normal.pdf, generated with
// reportlab — no client data) to make sure the pdftotext-first extraction
// path still returns clean, readable text for PDFs that always worked fine.
func TestExtractPdfStructured_NormalPDF(t *testing.T) {
	raw, highlighted := extractPdfStructured("testdata/sample_normal.pdf")
	if len(raw) == 0 {
		t.Fatal("expected non-empty extraction for sample_normal.pdf")
	}
	if len(raw) != len(highlighted) {
		t.Fatalf("raw/highlighted line count mismatch: %d vs %d", len(raw), len(highlighted))
	}

	joined := strings.Join(raw, "\n")
	if !strings.Contains(joined, "normal test document") {
		t.Errorf("expected recognizable extracted text, got:\n%s", joined)
	}
	if strings.Contains(joined, "PDF — no extractable text") {
		t.Errorf("normal PDF was incorrectly treated as having no extractable text:\n%s", joined)
	}
}
