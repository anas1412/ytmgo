package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// lipglossSGR is the escape lipgloss itself emits for c on the given
// layer — termenv's encoding, not sgrColor's.
func lipglossSGR(c lipgloss.TerminalColor, layer int) string {
	st := lipgloss.NewStyle()
	if layer == 38 {
		st = st.Foreground(c)
	} else {
		st = st.Background(c)
	}
	r := st.Render("x")
	i := strings.Index(r, "m")
	if i < 0 {
		return ""
	}
	return r[:i+1]
}

// TestSgrColorDiffersFromLipgloss records that the two encodings of one
// colour are not the same sequence, so nobody reads sgrColor's comment
// as a promise that they are. sgrColor takes RGBA()>>8, which is the
// scheme's hex byte exactly; termenv truncates uint8(f.R*255) off a
// float and lands one shade low whenever that product falls just under
// its integer. Neither is wrong on its own — they only matter together,
// which is what TestNoBackgroundSeam rules out.
func TestSgrColorDiffersFromLipgloss(t *testing.T) {
	old := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(old)

	seen, differ, total := map[string]bool{}, 0, 0
	for _, s := range schemes {
		for _, hex := range []string{
			s.bg, s.panel, s.hover, s.border, s.text, s.dim,
			s.mid, s.title, s.primary, s.active, s.danger, s.warn,
		} {
			if seen[hex] {
				continue
			}
			seen[hex] = true
			total++
			c := lipgloss.Color(hex)
			if sgrColor(c, 38) != lipglossSGR(c, 38) {
				differ++
			}
		}
	}
	// Not an equality the code depends on — a canary. If a lipgloss
	// upgrade rounds instead of truncating, this drops to zero and the
	// warning in sgrColor's comment can go with it.
	if differ == 0 {
		t.Errorf("lipgloss now agrees with sgrColor on all %d scheme colours; sgrColor's comment is stale", total)
	}
	t.Logf("%d of %d distinct scheme colours encode differently", differ, total)
}

// TestNoBackgroundSeam: sgrColor must be the only thing that paints the
// colours it paints. paintBg and fillRun lay colorBg and colorBgHover
// down as raw escapes; if a lipgloss style also rendered either as a
// Background, the same colour would reach the terminal as two sequences
// one shade apart and the fill would visibly change shade part-way
// along — the same class of break as the search field rendering in
// three fragments. Rendered frames are checked rather than the styles,
// since a background can arrive through a border, a Place or the
// textinput as easily as through an explicit Background call.
func TestNoBackgroundSeam(t *testing.T) {
	old := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(old)
	defer func() {
		detectedDark = true
		ApplyTheme(ThemeTerminal)
	}()

	for _, sc := range schemes {
		// InitialModel re-applies the saved theme, so the scheme has to
		// be forced after the model is built or the frame comes back
		// unpainted. A light terminal is the case that paints at all.
		m := worstCaseModel(t, 150, 40)
		m.npOn = false
		detectedDark = false
		ApplyTheme(sc.name)
		if !paintBackground {
			t.Fatalf("%s: expected to paint its own background", sc.name)
		}

		settings, focused := m, m
		settings.activePage = PageSettings
		focused.searchFocused = true
		frames := map[string]string{
			"stream":   m.View(),
			"settings": settings.View(),
			"search":   focused.View(),
		}

		for _, p := range []struct {
			role string
			c    lipgloss.TerminalColor
		}{{"colorBg", colorBg}, {"colorBgHover", colorBgHover}} {
			mine, theirs := sgrColor(p.c, 48), lipglossSGR(p.c, 48)
			if mine == theirs {
				continue // this colour survives termenv's truncation
			}
			for name, f := range frames {
				if strings.Contains(f, mine) && strings.Contains(f, theirs) {
					t.Errorf("%s/%s: %s painted both as %q and as %q — the fill changes shade mid-run",
						sc.name, name, p.role, mine, theirs)
				}
			}
		}
	}
}
