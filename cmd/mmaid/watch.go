package main

import (
	"fmt"
	"os"
	"time"
)

// watchInterval is how often --watch stats the file it renders.
const watchInterval = 250 * time.Millisecond

// runWatch renders the file, then re-renders it whenever its modification time
// changes. An editor that saves by renaming makes the file briefly absent, so a
// stat that fails is "no change yet" rather than the end. Ctrl-C ends it.
func runWatch(args []string, o outputSpec) {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "%smmaid:%s --watch needs a file to watch\n", ansiBold+ansiCyan, ansiReset)
		os.Exit(1)
	}
	path := args[0]

	var last time.Time
	for {
		if mtime, ok := changed(path, last); ok {
			data, err := os.ReadFile(path)
			if err == nil {
				last = mtime
				fmt.Print("\x1b[2J\x1b[H")
				fmt.Println(decorate(render(string(data), o), o))
			}
		}
		time.Sleep(watchInterval)
	}
}

// changed reports the file's modification time when it differs from the last
// one rendered, and whether there is anything to render.
func changed(path string, last time.Time) (time.Time, bool) {
	info, err := os.Stat(path)
	if err != nil {
		return last, false
	}
	mtime := info.ModTime()
	return mtime, !mtime.Equal(last)
}
