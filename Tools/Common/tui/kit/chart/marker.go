package chart

// MarkerGlyph returns the one-cell glyph that draws a marker of kind k in the current glyph mode.
// Sparkline-like widgets in other kit packages share it so a marker looks the same everywhere.
func MarkerGlyph(k MarkerKind) string {
	glyphs := [...]string{"┃", "│", "┊"}
	if ascii() {
		glyphs = [...]string{"|", ":", "*"}
	}
	if int(k) >= len(glyphs) {
		return glyphs[len(glyphs)-1]
	}
	return glyphs[k]
}
