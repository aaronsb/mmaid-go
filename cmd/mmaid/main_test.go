package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const source = "flowchart LR\n  A[Start] --> B[Finish]\n"

func TestOutputWritesTheRenderToAFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "diagram.txt")
	o := outputSpec{paddingX: 4, paddingY: 2, output: path}

	renderAndOutput(source, o)

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	got := string(data)
	if !strings.Contains(got, "Start") || !strings.Contains(got, "Finish") {
		t.Errorf("file holds no diagram:\n%s", got)
	}
	if !strings.HasSuffix(got, "\n") {
		t.Error("the file should end in a newline, as stdout would")
	}

	// A second render truncates rather than appends.
	renderAndOutput("flowchart LR\n  C[Only]\n", o)
	data, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "Start") {
		t.Error("--output should truncate the file it writes")
	}
}

func TestOutputHonoursMarkdown(t *testing.T) {
	path := filepath.Join(t.TempDir(), "diagram.md")
	renderAndOutput(source, outputSpec{paddingX: 4, paddingY: 2, output: path, markdown: true})

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(data), "```\n") || !strings.HasSuffix(string(data), "```\n") {
		t.Errorf("markdown fences missing:\n%s", data)
	}
}

func TestChangedWaitsForAMissingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "watched.mmd")

	// An editor that saves by renaming leaves the file briefly absent; that is
	// not a reason to stop watching.
	if _, ok := changed(path, time.Time{}); ok {
		t.Error("a missing file should report no change")
	}

	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	mtime, ok := changed(path, time.Time{})
	if !ok {
		t.Fatal("a file that appeared should report a change")
	}
	if _, ok := changed(path, mtime); ok {
		t.Error("an unchanged file should report no change")
	}
}
