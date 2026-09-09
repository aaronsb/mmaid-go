package renderer

import (
	"strings"
	"testing"
)

func TestLinkLayerSurvivesResize(t *testing.T) {
	c := NewCanvas(3, 1)
	c.SetLink(0, 2, "https://example.com/a")

	c.Resize(6, 2)
	if got := c.GetLink(0, 2); got != "https://example.com/a" {
		t.Errorf("link after resize = %q", got)
	}
	if got := c.GetLink(1, 5); got != "" {
		t.Errorf("a new cell should carry no link, got %q", got)
	}

	c.SetLink(1, 5, "https://example.com/b")
	if got := c.GetLink(1, 5); got != "https://example.com/b" {
		t.Errorf("link in a grown cell = %q", got)
	}

	c.ClearCell(0, 2)
	if got := c.GetLink(0, 2); got != "" {
		t.Errorf("a cleared cell should carry no link, got %q", got)
	}
}

func TestCurrentLinkStampsTextOnly(t *testing.T) {
	c := NewCanvas(12, 1)
	c.SetCurrentLink("https://example.com/a")
	c.PutText(0, 0, "ab", "label")
	c.SetCurrentLink("")
	c.PutText(0, 4, "cd", "label")

	if c.GetLink(0, 0) == "" || c.GetLink(0, 1) == "" {
		t.Error("text written under a current link should carry it")
	}
	if c.GetLink(0, 4) != "" || c.GetLink(0, 5) != "" {
		t.Error("text written after the link was cleared should carry none")
	}
}

func TestWideRuneCarriesTheLinkOnBothCells(t *testing.T) {
	c := NewCanvas(6, 1)
	c.SetCurrentLink("https://example.com/a")
	c.PutText(0, 0, "日", "label")
	c.SetCurrentLink("")

	if c.GetLink(0, 0) == "" || c.GetLink(0, 1) == "" {
		t.Error("a wide rune's continuation cell should carry the link too")
	}

	c.SetHyperlinks(true)
	out := c.ToString()
	if !strings.Contains(out, "\033]8;;https://example.com/a\033\\日\033]8;;\033\\") {
		t.Errorf("the pair does not bracket the wide rune: %q", out)
	}
}

func TestUnsafeURLIsNotEmitted(t *testing.T) {
	c := NewCanvas(4, 1)
	c.SetCurrentLink("https://x/\033]0;PWNED\a")
	c.PutText(0, 0, "ab", "label")
	c.SetCurrentLink("")
	c.SetHyperlinks(true)

	out := c.ToString()
	if strings.ContainsAny(out, "\033\a") {
		t.Errorf("an escape reached the output: %q", out)
	}
}
