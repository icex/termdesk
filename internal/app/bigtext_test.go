package app

import (
	"image/color"
	"testing"
)

func TestTextWidths(t *testing.T) {
	if got := miniTextWidth("A"); got != miniCharWidth {
		t.Fatalf("miniTextWidth(A)=%d want %d", got, miniCharWidth)
	}
	if got := miniTextWidth("aA"); got != miniCharWidth*2+1 {
		t.Fatalf("miniTextWidth(aA)=%d want %d", got, miniCharWidth*2+1)
	}
	if got := bigTextWidth("A"); got != bigCharWidth {
		t.Fatalf("bigTextWidth(A)=%d want %d", got, bigCharWidth)
	}
	if got := bigTextWidth("aA"); got != bigCharWidth*2+1 {
		t.Fatalf("bigTextWidth(aA)=%d want %d", got, bigCharWidth*2+1)
	}
}

func TestRenderMiniText(t *testing.T) {
	buf := NewBuffer(10, 4, "#000000")
	renderMiniText(buf, 0, 0, "A", color.White, color.Black)
	found := false
	for y := 0; y < buf.Height; y++ {
		for x := 0; x < buf.Width; x++ {
			if buf.Cells[y][x].Char != ' ' {
				found = true
				break
			}
		}
	}
	if !found {
		t.Fatalf("renderMiniText produced only spaces")
	}
}

func TestRenderBigText(t *testing.T) {
	buf := NewBuffer(10, 6, "#000000")
	renderBigText(buf, 0, 0, "A", color.White, color.Black)
	found := false
	for y := 0; y < buf.Height; y++ {
		for x := 0; x < buf.Width; x++ {
			if buf.Cells[y][x].Char != ' ' {
				found = true
				break
			}
		}
	}
	if !found {
		t.Fatalf("renderBigText produced only spaces")
	}
}
