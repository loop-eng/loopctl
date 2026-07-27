package panel

import (
	"sort"

	"charm.land/bubbles/v2/table"
)

// colSpec describes one table column's minimum and ideal width, plus a
// shrink priority used when the ideal widths don't fit the terminal.
type colSpec struct {
	title    string
	min      int
	ideal    int
	priority int // shrink order, ascending — lower priority shrinks first
}

// fitColumns lays out columns to sum to at most totalWidth wherever
// possible, shrinking lower-priority columns toward their minimum first.
// This is what keeps the session/history tables from silently overflowing
// on terminals narrower than the columns' combined ideal width (GitHub
// issue #6). If every column is already at its minimum and the row still
// doesn't fit, the columns are left at their minimums — below that floor,
// something has to give, and cutting further would make the cells
// unreadable rather than merely narrow.
func fitColumns(totalWidth int, specs []colSpec) []table.Column {
	widths := make([]int, len(specs))
	for i, s := range specs {
		widths[i] = s.ideal
	}

	sum := func() int {
		total := 0
		for _, w := range widths {
			total += w
		}
		return total
	}

	order := make([]int, len(specs))
	for i := range order {
		order[i] = i
	}
	sort.Slice(order, func(i, j int) bool {
		return specs[order[i]].priority < specs[order[j]].priority
	})

	for _, idx := range order {
		for sum() > totalWidth && widths[idx] > specs[idx].min {
			widths[idx]--
		}
	}

	cols := make([]table.Column, len(specs))
	for i, s := range specs {
		cols[i] = table.Column{Title: s.title, Width: widths[i]}
	}
	return cols
}
