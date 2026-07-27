package style

import (
	"strings"
	"testing"
)

func TestSparkline_EmptyInput_NoPanic(t *testing.T) {
	got := Sparkline(nil, 10)
	if len(stripANSI(got)) != 10 {
		t.Errorf("expected blank sparkline of width 10, got %q (len %d)", got, len(stripANSI(got)))
	}
}

func TestSparkline_ZeroWidth(t *testing.T) {
	if got := Sparkline([]float64{1, 2, 3}, 0); got != "" {
		t.Errorf("expected empty string for zero width, got %q", got)
	}
}

func TestSparkline_SingleValue_RightAligned(t *testing.T) {
	got := stripANSI(Sparkline([]float64{5.0}, 5))
	runes := []rune(got)
	if len(runes) != 5 {
		t.Fatalf("expected 5 runes, got %d", len(runes))
	}
	if runes[4] == ' ' {
		t.Error("expected the rightmost glyph to be non-blank for a single value")
	}
	for i := 0; i < 4; i++ {
		if runes[i] != ' ' {
			t.Errorf("expected left padding at position %d, got %q", i, string(runes[i]))
		}
	}
}

func TestSparkline_AllZero_NoDivideByZero(t *testing.T) {
	got := Sparkline([]float64{0, 0, 0}, 3)
	if got == "" {
		t.Error("expected non-empty sparkline for all-zero input")
	}
}

func TestSparkline_DownsamplesWhenMoreValuesThanWidth(t *testing.T) {
	values := make([]float64, 100)
	for i := range values {
		values[i] = float64(i)
	}
	got := stripANSI(Sparkline(values, 10))
	if len([]rune(got)) != 10 {
		t.Errorf("expected downsampled width of 10, got %d", len([]rune(got)))
	}
}

func TestOutcomeStyleFor_KnownAndUnknown(t *testing.T) {
	for _, o := range []string{"Done", "Failed", "Killed", "Warned", "SomethingElse"} {
		s := OutcomeStyleFor(o)
		if s.Render("x") == "" {
			t.Errorf("expected non-empty render for outcome %q", o)
		}
	}
}

// stripANSI removes lipgloss/ANSI escape sequences so tests can assert on
// visible character count and content.
func stripANSI(s string) string {
	var b strings.Builder
	inEscape := false
	for _, r := range s {
		if r == '\x1b' {
			inEscape = true
			continue
		}
		if inEscape {
			if r == 'm' {
				inEscape = false
			}
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}
