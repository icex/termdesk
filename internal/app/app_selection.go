package app

import (
	"encoding/base64"
	"os"
	"strings"

	"github.com/icex/termdesk/internal/terminal"
	"github.com/icex/termdesk/pkg/geometry"
)

// mouseToAbsLine converts a content-area row to an absolute line number.
// absLine 0 = oldest scrollback line, scrollbackLen = emulator row 0.
func mouseToAbsLine(contentRow, scrollOffset, scrollbackLen, contentH int) int {
	scrollLines := scrollOffset
	if scrollLines > contentH {
		scrollLines = contentH
	}
	if contentRow < scrollLines {
		// In scrollback region: row 0 shows scrollback[scrollOffset-1]
		return scrollbackLen - scrollOffset + contentRow
	}
	// In emulator region
	emuRow := contentRow - scrollLines
	return scrollbackLen + emuRow
}

// absLineToContentRow converts an absolute line to a content-area row (-1 if not visible).
func absLineToContentRow(absLine, scrollOffset, scrollbackLen, contentH int) int {
	scrollLines := scrollOffset
	if scrollLines > contentH {
		scrollLines = contentH
	}
	if absLine < scrollbackLen {
		// Scrollback line
		sbOffset := scrollbackLen - 1 - absLine // offset from most recent
		if sbOffset < scrollOffset && sbOffset >= scrollOffset-scrollLines {
			return scrollLines - 1 - (sbOffset - (scrollOffset - scrollLines))
		}
		// Simpler: the visible scrollback range is absLine in [scrollbackLen-scrollOffset, scrollbackLen-scrollOffset+scrollLines-1]
		row := absLine - (scrollbackLen - scrollOffset)
		if row >= 0 && row < scrollLines {
			return row
		}
		return -1
	}
	// Emulator line
	emuRow := absLine - scrollbackLen
	row := scrollLines + emuRow
	if row >= 0 && row < contentH {
		return row
	}
	return -1
}

// extractSelText extracts text from the terminal between two selection points.
func extractSelText(term *terminal.Terminal, start, end geometry.Point) string {
	return extractSelTextWithSnapshot(term, nil, start, end)
}

func extractSelTextWithSnapshot(term *terminal.Terminal, snap *CopySnapshot, start, end geometry.Point) string {
	sbLen := term.ScrollbackLen()
	termW := term.Width()
	termH := term.Height()
	if snap != nil {
		sbLen = snap.ScrollbackLen()
		termW = snap.Width
		termH = snap.Height
	}
	totalLines := sbLen + termH

	// Normalize: ensure start <= end
	sLine, sCol := start.Y, start.X
	eLine, eCol := end.Y, end.X
	if sLine > eLine || (sLine == eLine && sCol > eCol) {
		sLine, eLine = eLine, sLine
		sCol, eCol = eCol, sCol
	}

	var lines []string
	for line := sLine; line <= eLine && line < totalLines; line++ {
		if line < 0 {
			continue
		}
		colStart := 0
		colEnd := termW
		if line == sLine {
			colStart = sCol
		}
		if line == eLine {
			colEnd = eCol + 1
		}
		if colEnd > termW {
			colEnd = termW
		}
		if colStart < 0 {
			colStart = 0
		}

		var row strings.Builder
		if line < sbLen {
			sbOffset := sbLen - 1 - line
			cells := term.ScrollbackLine(sbOffset)
			if snap != nil {
				cells = snap.ScrollbackLine(sbOffset)
			}
			for col := colStart; col < colEnd && col < len(cells); col++ {
				if cells[col].Content != "" {
					row.WriteString(cells[col].Content)
				} else {
					row.WriteByte(' ')
				}
			}
		} else {
			emuRow := line - sbLen
			if emuRow < termH {
				if snap != nil {
					var cells []terminal.ScreenCell
					if emuRow < len(snap.Screen) {
						cells = snap.Screen[emuRow]
					}
					for col := colStart; col < colEnd; col++ {
						if col < len(cells) && cells[col].Content != "" {
							row.WriteString(cells[col].Content)
						} else {
							row.WriteByte(' ')
						}
					}
				} else {
					for col := colStart; col < colEnd; col++ {
						cell := term.CellAt(col, emuRow)
						if cell != nil && cell.Content != "" {
							row.WriteString(cell.Content)
						} else {
							row.WriteByte(' ')
						}
					}
				}
			}
		}
		lines = append(lines, strings.TrimRight(row.String(), " "))
	}
	return strings.Join(lines, "\n")
}

// writeOSC52 writes text to the system clipboard via OSC 52 escape sequence.
func writeOSC52(text string) {
	b64 := base64.StdEncoding.EncodeToString([]byte(text))
	var seq []byte
	seq = append(seq, "\x1b]52;c;"...)
	seq = append(seq, b64...)
	seq = append(seq, '\x07')
	os.Stdout.Write(seq)
}
