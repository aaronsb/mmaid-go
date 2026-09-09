package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/aaronsb/mmaid-go/internal/config"
	"github.com/aaronsb/mmaid-go/internal/glyph"
	"github.com/aaronsb/mmaid-go/internal/renderer"
	"github.com/aaronsb/mmaid-go/internal/tester"
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
		case "glyphs":
			s.Glyphs = &value
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
		runConfigInit(args[1:])
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
	fmt.Fprintf(w, "    %sinit%s   Probe the terminal, ask which glyph families look wrong, write its profile\n", ansiYellow, ansiReset)
	fmt.Fprintf(w, "           %s--no-probe%s asks only; %s--force%s replaces an existing profile without asking\n\n", ansiYellow, ansiReset, ansiYellow, ansiReset)
}

// glyphSet returns the built-in set a name selects, warning on stderr and
// using unicode when the name is unknown.
func glyphSet(name string) glyph.Set {
	set, ok := glyph.LookupSet(name)
	if !ok {
		fmt.Fprintf(os.Stderr, "%smmaid:%s unknown glyph set %q (one of %s); using unicode\n",
			ansiBold+ansiCyan, ansiReset, name, strings.Join(glyph.SetNames, ", "))
		return glyph.DefaultSet()
	}
	return set
}

// warnUnknownFamilies reports failed-list entries that name no family.
func warnUnknownFamilies(failed []string) {
	_, unknown := glyph.ParseFamilies(failed)
	for _, name := range unknown {
		fmt.Fprintf(os.Stderr, "%smmaid:%s failed: %q is not a glyph family, ignored\n", ansiBold+ansiCyan, ansiReset, name)
	}
}

// runConfigInit is the tester: probe, ask, write, show.
func runConfigInit(args []string) {
	fs := flag.NewFlagSet("config init", flag.ExitOnError)
	noProbe := fs.Bool("no-probe", false, "ask only; do not query the terminal")
	force := fs.Bool("force", false, "replace an existing profile without asking")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: mmaid config init [--no-probe] [--force]")
	}
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}
	fail := func(err error) {
		fmt.Fprintf(os.Stderr, "%smmaid:%s config init: %v\n", ansiBold+ansiCyan, ansiReset, err)
		os.Exit(1)
	}

	res, path, err := resolveSettings()
	if err != nil {
		fail(err)
	}
	if path == "" {
		fail(fmt.Errorf("nowhere to write: neither XDG_CONFIG_HOME nor HOME is set"))
	}
	// The sheet shows the set as it draws with nothing failed, so a re-run
	// tests the families rather than their fallbacks.
	samples := renderer.GlyphSamples(glyphSet(res.Glyphs))

	probe := tester.FromEnv()
	if !*noProbe {
		probe = tester.Probe(os.Stdin, os.Stdout, samples)
	}
	if probe.Identity == "" {
		fail(fmt.Errorf("no terminal identity: neither TERM_PROGRAM nor TERM is set"))
	}
	fmt.Printf("terminal   %s\n", probe.Identity)
	fmt.Printf("truecolor  %t (COLORTERM)\n", probe.Truecolor)
	switch {
	case *noProbe:
		fmt.Println("probes     skipped (--no-probe)")
	case probe.Probed:
		fmt.Printf("probes     advance width checked for %d families\n", len(samples)-1)
	default:
		fmt.Println("probes     skipped (not a terminal, or it did not answer)")
	}
	fmt.Println()

	failures, err := tester.Ask(os.Stdin, os.Stdout, samples, probe.Failed)
	if err != nil {
		fail(err)
	}
	tester.Report(os.Stdout, failures)
	fmt.Println()

	file, err := config.Load(path)
	if err != nil {
		fail(err)
	}
	if _, exists := file.Profiles[probe.Identity]; exists && !*force {
		fmt.Printf("Profile %q exists in %s. Replace its truecolor and failed keys? [y/N]: ", probe.Identity, path)
		answer, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		if a := strings.ToLower(strings.TrimSpace(answer)); a != "y" && a != "yes" {
			fmt.Println("left as it was")
			return
		}
	}
	tester.Merge(&file, probe.Identity, probe.Truecolor, failures)
	if err := config.Save(path, file); err != nil {
		fail(err)
	}
	fmt.Printf("wrote %s\n", path)
	if rendered := config.TerminalIdentity(); rendered != probe.Identity {
		fmt.Printf("note: a render looks its profile up by TERM_PROGRAM or TERM, which is %q here\n", rendered)
	}
	fmt.Println()
	printConfigShow()
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
