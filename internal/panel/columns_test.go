package panel

import (
	"testing"

	"charm.land/bubbles/v2/table"
)

func totalWidth(cols []table.Column) int {
	sum := 0
	for _, c := range cols {
		sum += c.Width
	}
	return sum
}

// TestSessionColumnsFitNarrowTerminals is a regression test for GitHub
// issue #6: the session table's fixed column widths used to sum to ~84
// columns regardless of the actual terminal size, silently overflowing any
// narrower terminal. defaultColumns must now shrink to fit.
// assertFitsOrAtFloor checks the fit-or-give-up contract: either the
// columns sum to no more than the terminal width, or (below the practical
// floor) every column is already at its own minimum — there is nothing
// more the layout can do at that point without making cells unreadable.
func assertFitsOrAtFloor(t *testing.T, label string, w int, cols []table.Column, mins []int) {
	t.Helper()
	sum := totalWidth(cols)
	if sum <= w {
		return
	}
	for i, c := range cols {
		if c.Width > mins[i] {
			t.Errorf("%s(%d): column %q is %d, above its floor %d, yet total %d still exceeds width %d",
				label, w, c.Title, c.Width, mins[i], sum, w)
		}
	}
}

func TestSessionColumnsFitNarrowTerminals(t *testing.T) {
	mins := []int{7, 12, 10, 7, 5, 7, 7, 6} // Status, Project, Model, Duration, Iters, Cost, Context, Tool/min
	for _, w := range []int{40, 50, 60, 70, 80, 83, 84, 90, 120, 200} {
		cols := defaultColumns(w)
		assertFitsOrAtFloor(t, "defaultColumns", w, cols, mins)
		for _, c := range cols {
			if c.Width <= 0 {
				t.Errorf("defaultColumns(%d): column %q has non-positive width %d", w, c.Title, c.Width)
			}
		}
	}
}

func TestHistoryColumnsFitNarrowTerminals(t *testing.T) {
	mins := []int{12, 10, 8, 7, 5, 7, 7} // Project, Model, Ended, Duration, Iters, Cost, Outcome
	for _, w := range []int{40, 50, 60, 70, 80, 83, 84, 90, 120, 200} {
		cols := historyColumns(w)
		assertFitsOrAtFloor(t, "historyColumns", w, cols, mins)
		for _, c := range cols {
			if c.Width <= 0 {
				t.Errorf("historyColumns(%d): column %q has non-positive width %d", w, c.Title, c.Width)
			}
		}
	}
}

// TestColumnsFitAtRealisticFloor confirms the practical minimum supported
// terminal width for each table is well under the old fixed ~84/~78-column
// requirement — the concrete fix for GitHub issue #6.
func TestColumnsFitAtRealisticFloor(t *testing.T) {
	if sum := totalWidth(defaultColumns(65)); sum > 65 {
		t.Errorf("session table should fit at 65 columns, got width sum %d", sum)
	}
	if sum := totalWidth(historyColumns(60)); sum > 60 {
		t.Errorf("history table should fit at 60 columns, got width sum %d", sum)
	}
}

func TestFitColumnsRespectsMinimums(t *testing.T) {
	specs := []colSpec{
		{title: "A", min: 5, ideal: 20, priority: 0},
		{title: "B", min: 5, ideal: 20, priority: 1},
	}
	cols := fitColumns(1, specs)
	for i, c := range cols {
		if c.Width != specs[i].min {
			t.Errorf("column %q = %d, want min %d when width is far too small", c.Title, c.Width, specs[i].min)
		}
	}
}

func TestFitColumnsUsesIdealWhenRoomAllows(t *testing.T) {
	specs := []colSpec{
		{title: "A", min: 5, ideal: 10, priority: 0},
		{title: "B", min: 5, ideal: 10, priority: 1},
	}
	cols := fitColumns(100, specs)
	for i, c := range cols {
		if c.Width != specs[i].ideal {
			t.Errorf("column %q = %d, want ideal %d when width is plentiful", c.Title, c.Width, specs[i].ideal)
		}
	}
}
