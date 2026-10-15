package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/mattn/go-runewidth"
)

// cellWidth measures terminal cells. Ambiguous-width characters count as one
// cell, as in most terminals; the tables avoid them anyway.
var cellWidth = func() *runewidth.Condition {
	c := runewidth.NewCondition()
	c.EastAsianWidth = false
	return c
}()

// printTable writes rows as aligned columns separated by two spaces. Widths
// are measured in terminal cells, so CJK text lines up (text/tabwriter counts
// runes, which misaligns full-width characters).
func printTable(w io.Writer, rows [][]string) {
	var widths []int
	for _, r := range rows {
		for i, cell := range r {
			if i >= len(widths) {
				widths = append(widths, 0)
			}
			widths[i] = max(widths[i], cellWidth.StringWidth(cell))
		}
	}
	for _, r := range rows {
		var b strings.Builder
		for i, cell := range r {
			if i == len(r)-1 {
				b.WriteString(cell)
				break
			}
			b.WriteString(cellWidth.FillRight(cell, widths[i]))
			b.WriteString("  ")
		}
		fmt.Fprintln(w, strings.TrimRight(b.String(), " "))
	}
}

// splitHeader turns a tab-separated translated header into cells.
func splitHeader(s string) []string { return strings.Split(s, "\t") }
