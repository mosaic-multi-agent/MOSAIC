package artifact

import (
	"errors"
	"strings"

	"mosaic-run/internal/domain"
)

// Section names used in damage problems.
const sectionFrontmatter = "frontmatter"

// damage collects every problem found while reading one document, so the reader
// reports all of them instead of stopping at the first.
type damage struct {
	frontmatter []domain.ArtifactDamageProblem
	body        []domain.ArtifactDamageProblem
	recorded    *domain.ArtifactState // frontmatter values, kept when the frontmatter itself is intact
}

func (d *damage) addFrontmatter(line int, problem string) {
	d.frontmatter = append(d.frontmatter, domain.ArtifactDamageProblem{Section: sectionFrontmatter, Line: line, Problem: problem})
}

func (d *damage) addBody(section string, line int, problem string) {
	d.body = append(d.body, domain.ArtifactDamageProblem{Section: section, Line: line, Problem: problem})
}

// addRefusal records a refusal-shaped frontmatter value error (the helpers that
// validate settings and selections still describe their findings that way) as a
// frontmatter problem at line.
func (d *damage) addRefusal(line int, err error) {
	var re *domain.RefusalError
	if errors.As(err, &re) {
		d.addFrontmatter(line, re.Reason)
		return
	}
	d.addFrontmatter(line, err.Error())
}

func (d *damage) frontmatterIntact() bool { return len(d.frontmatter) == 0 }

func (d *damage) found() bool { return len(d.frontmatter) > 0 || len(d.body) > 0 }

// err builds the damage error from everything collected, frontmatter problems
// first and then the body sections in file order.
func (d *damage) err() *domain.ArtifactDamageError {
	problems := append(append([]domain.ArtifactDamageProblem(nil), d.frontmatter...), d.body...)
	// recorded is set only while the frontmatter had no problem of its own; the
	// later last_agent check does not count.
	return &domain.ArtifactDamageError{Problems: problems, Frontmatter: d.recorded}
}

// frontmatterLine returns the file line of the first frontmatter line whose key
// is key, indented or not; 0 when there is none. The frontmatter content starts
// on file line 2, after the opening delimiter.
func frontmatterLine(fmContent, key string) int {
	for i, l := range strings.Split(fmContent, "\n") {
		if strings.HasPrefix(strings.TrimSpace(l), key+":") {
			return i + 2
		}
	}
	return 0
}

// checkFrontmatterSyntax reports values that open a flow sequence or mapping
// and never close it. The minimal frontmatter reader would otherwise take such
// a line for an ordinary text value.
func checkFrontmatterSyntax(fmContent string, d *damage) {
	for i, raw := range strings.Split(fmContent, "\n") {
		line := strings.TrimSpace(strings.TrimRight(raw, "\r"))
		idx := strings.Index(line, ": ")
		if idx <= 0 || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "- ") {
			continue
		}
		key, value := line[:idx], strings.TrimSpace(line[idx+2:])
		var closer string
		switch {
		case strings.HasPrefix(value, "["):
			closer = "]"
		case strings.HasPrefix(value, "{"):
			closer = "}"
		default:
			continue
		}
		if !strings.HasSuffix(value, closer) {
			d.addFrontmatter(i+2, "value of '"+key+"' opens "+value[:1]+" without closing "+closer)
		}
	}
}
