package domain

import "testing"

func TestGuidance_Normalized(t *testing.T) {
	cases := []struct {
		name string
		in   Guidance
		want Guidance
	}{
		{"crlf and lone cr become lf", Guidance{"a\r\nb\rc", "d\r\ne"}, Guidance{"a\nb\nc", "d\ne"}},
		{"trailing newlines removed from each field", Guidance{"a\n\n\n", "b\n"}, Guidance{"a", "b"}},
		{"trailing crlf removed", Guidance{"a\r\n\r\n", ""}, Guidance{"a", ""}},
		{"leading and inner text unchanged", Guidance{"\n  a\n\nb", " \nc"}, Guidance{"\n  a\n\nb", " \nc"}},
		{"only newlines becomes empty", Guidance{"\n\n", "\r\n"}, Guidance{}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.in.Normalized(); got != c.want {
				t.Errorf("want %q, got %q", c.want, got)
			}
		})
	}
}
