package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"

	"github.com/aaronsb/mmaid-go/internal/config"
)

// flagSettings reads the settings the command line actually named. A flag left
// at its default is silent, so the layers below it can speak.
func flagSettings() config.Settings {
	var s config.Settings
	flag.Visit(func(f *flag.Flag) {
		value := f.Value.String()
		switch f.Name {
		case "a", "ascii":
			// --ascii=false is a request for Unicode, and outranks a profile
			// that asked for ASCII.
			glyphs := "unicode"
			if value == "true" {
				glyphs = "ascii"
			}
			s.Glyphs = &glyphs
		case "t", "theme":
			s.Theme = &value
		case "w", "width":
			if n, err := strconv.Atoi(value); err == nil {
				s.Width = &n
			}
		case "orientation":
			s.Orientation = &value
		case "padding-x":
			if n, err := strconv.Atoi(value); err == nil {
				s.PaddingX = &n
			}
		case "padding-y":
			if n, err := strconv.Atoi(value); err == nil {
				s.PaddingY = &n
			}
		case "sharp-edges":
			on := value == "true"
			s.SharpEdges = &on
		}
	})
	return s
}

// resolveSettings loads the configuration file and resolves every setting
// against it, reporting the path it read. A file that cannot be read yields the
// error alongside a resolution over the layers that are left, so a caller may
// warn and carry on.
func resolveSettings() (config.Resolved, string, error) {
	path := config.Path()
	file, err := config.Load(path)
	return config.Resolve(flagSettings(), os.Getenv, file, config.TerminalIdentity()), path, err
}

// runConfig handles the `config` subcommand.
func runConfig(args []string) {
	if len(args) == 0 {
		printConfigUsage()
		os.Exit(2)
	}
	switch args[0] {
	case "show":
		printConfigShow()
	case "init":
		fmt.Fprintln(os.Stderr, "not implemented yet: see ADR-500")
		os.Exit(2)
	default:
		fmt.Fprintf(os.Stderr, "%smmaid:%s config: unknown subcommand %q\n", ansiBold+ansiCyan, ansiReset, args[0])
		printConfigUsage()
		os.Exit(2)
	}
}

func printConfigUsage() {
	w := os.Stderr
	fmt.Fprintf(w, "\n  %smmaid config%s\n\n", ansiBold+ansiCyan, ansiReset)
	fmt.Fprintf(w, "    %sshow%s   Print every setting with its resolved value and source\n", ansiYellow, ansiReset)
	fmt.Fprintf(w, "    %sinit%s   Probe the terminal and write its profile (not implemented yet)\n\n", ansiYellow, ansiReset)
}

// printConfigShow prints the resolved value and the source of every setting.
// This is the one command whose subject is the configuration, so a file it
// cannot read is fatal here and a warning everywhere else.
func printConfigShow() {
	res, path, err := resolveSettings()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%smmaid:%s config: %v\n", ansiBold+ansiCyan, ansiReset, err)
		os.Exit(1)
	}

	switch _, statErr := os.Stat(path); {
	case path == "":
		fmt.Println("file      (none: neither XDG_CONFIG_HOME nor HOME is set)")
	case statErr != nil:
		fmt.Printf("file      %s (no config file)\n", path)
	default:
		fmt.Printf("file      %s\n", path)
	}
	identity := config.TerminalIdentity()
	if identity == "" {
		identity = "(unknown)"
	}
	fmt.Printf("identity  %s\n\n", identity)

	nameWidth, valueWidth := 0, 0
	for _, key := range config.Keys {
		nameWidth = max(nameWidth, len(key))
		valueWidth = max(valueWidth, len(res.Value(key)))
	}
	for _, key := range config.Keys {
		fmt.Printf("%-*s  %-*s  (%s)\n", nameWidth, key, valueWidth, res.Value(key), res.Source[key])
	}
	for _, warning := range res.Warnings {
		fmt.Fprintf(os.Stderr, "%smmaid:%s config: %s\n", ansiBold+ansiCyan, ansiReset, warning)
	}
}
