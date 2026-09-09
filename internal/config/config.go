// Package config reads mmaid's XDG configuration file and resolves each
// setting from the command line, the environment, the terminal's profile, the
// file's default section, and the built-in defaults, in that order.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// Settings is one layer of configuration. A nil field is unset, so a layer can
// supply one key without claiming the rest.
type Settings struct {
	Theme         *string  `json:"theme,omitempty"`
	Glyphs        *string  `json:"glyphs,omitempty"`
	Width         *int     `json:"width,omitempty"`
	Orientation   *string  `json:"orientation,omitempty"`
	PaddingX      *int     `json:"padding_x,omitempty"`
	PaddingY      *int     `json:"padding_y,omitempty"`
	SharpEdges    *bool    `json:"sharp_edges,omitempty"`
	Truecolor     *bool    `json:"truecolor,omitempty"`
	Hyperlinks    *bool    `json:"hyperlinks,omitempty"`
	AmbiguousWide *bool    `json:"ambiguous_wide,omitempty"`
	Failed        []string `json:"failed,omitempty"`
}

// File is the on-disk shape: a default section and one section per terminal
// identity.
type File struct {
	Default  Settings            `json:"default,omitempty"`
	Profiles map[string]Settings `json:"profiles,omitempty"`
}

// Setting names, used as map keys in Resolved.Source and as the labels
// `mmaid config show` prints.
const (
	KeyTheme         = "theme"
	KeyGlyphs        = "glyphs"
	KeyWidth         = "width"
	KeyOrientation   = "orientation"
	KeyPaddingX      = "padding_x"
	KeyPaddingY      = "padding_y"
	KeySharpEdges    = "sharp_edges"
	KeyTruecolor     = "truecolor"
	KeyHyperlinks    = "hyperlinks"
	KeyAmbiguousWide = "ambiguous_wide"
	KeyFailed        = "failed"
)

// Keys lists every setting in the order `mmaid config show` prints them.
var Keys = []string{
	KeyTheme, KeyGlyphs, KeyWidth, KeyOrientation,
	KeyPaddingX, KeyPaddingY, KeySharpEdges,
	KeyTruecolor, KeyHyperlinks, KeyAmbiguousWide, KeyFailed,
}

// envName returns the environment variable a setting reads.
func envName(key string) string { return "MMAID_" + strings.ToUpper(key) }

// Built-in defaults, the last resort of every resolution.
const (
	defaultGlyphs   = "unicode"
	defaultPaddingX = 4
	defaultPaddingY = 2
)

// Resolved holds the final value of every setting and where each came from.
type Resolved struct {
	Theme         string
	Glyphs        string
	Width         int
	Orientation   string
	PaddingX      int
	PaddingY      int
	SharpEdges    bool
	Truecolor     bool
	Hyperlinks    bool
	AmbiguousWide bool
	Failed        []string

	// Source maps a setting name to the layer that supplied it: "flag",
	// "env MMAID_X", "profile <identity>", "default", "builtin", or
	// "NO_COLOR" for a theme NO_COLOR suppressed.
	Source map[string]string

	// Warnings names environment values that could not be parsed.
	Warnings []string
}

// Value returns a setting's resolved value formatted for display.
func (r Resolved) Value(key string) string {
	switch key {
	case KeyTheme:
		if r.Theme == "" {
			return "(none)"
		}
		return r.Theme
	case KeyGlyphs:
		return r.Glyphs
	case KeyWidth:
		if r.Width == 0 {
			return "auto"
		}
		return strconv.Itoa(r.Width)
	case KeyOrientation:
		if r.Orientation == "" {
			return "(from source)"
		}
		return r.Orientation
	case KeyPaddingX:
		return strconv.Itoa(r.PaddingX)
	case KeyPaddingY:
		return strconv.Itoa(r.PaddingY)
	case KeySharpEdges:
		return strconv.FormatBool(r.SharpEdges)
	case KeyTruecolor:
		return strconv.FormatBool(r.Truecolor)
	case KeyHyperlinks:
		return strconv.FormatBool(r.Hyperlinks)
	case KeyAmbiguousWide:
		return strconv.FormatBool(r.AmbiguousWide)
	case KeyFailed:
		if len(r.Failed) == 0 {
			return "(none)"
		}
		return strings.Join(r.Failed, ",")
	}
	return ""
}

// Path returns the configuration file's location:
// $XDG_CONFIG_HOME/mmaid/config.json, or ~/.config/mmaid/config.json. With
// neither variable set there is nowhere to look and it returns "", which Load
// reads as no file rather than as a path relative to the working directory.
func Path() string {
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, "mmaid", "config.json")
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, ".config", "mmaid", "config.json")
}

// Load reads the file at path. A path that names nothing — missing, or a
// directory, or under one — is an empty configuration and no error. Malformed
// JSON, a wrongly typed value and an unreadable file are errors naming the
// path, and the caller decides whether one is fatal.
func Load(path string) (File, error) {
	var f File
	if path == "" {
		return f, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if isAbsent(err) {
			return f, nil
		}
		return f, fmt.Errorf("reading %s: %w", path, err)
	}
	if err := json.Unmarshal(data, &f); err != nil {
		return File{}, fmt.Errorf("%s: %w", path, err)
	}
	return f, nil
}

// isAbsent reports whether the error means there is no configuration file
// there: no such path, a directory in place of the file, or a file where a
// directory was expected.
func isAbsent(err error) bool {
	return os.IsNotExist(err) ||
		errors.Is(err, syscall.EISDIR) ||
		errors.Is(err, syscall.ENOTDIR)
}

// TerminalIdentity returns the key a profile is looked up by: TERM_PROGRAM when
// the terminal sets it, else TERM.
func TerminalIdentity() string {
	if p := os.Getenv("TERM_PROGRAM"); p != "" {
		return p
	}
	return os.Getenv("TERM")
}

// identityFromDA1 will map a DA1 response (ESC [ c) to a terminal name. The
// probe needs raw mode, which arrives with the guided tester in the second half
// of ADR-500; until then it reports nothing and TerminalIdentity falls back to
// the environment.
func identityFromDA1() string { return "" }

// resolver walks the layers for one setting and records where the answer came
// from.
type resolver struct {
	env      func(string) string
	identity string
	out      *Resolved
}

// candidate is one layer's answer for a setting.
type candidate struct{ value, source string }

// candidates lists the layers that answered for one setting, in resolution
// order: the flag, the environment, the profile, the file's default section.
func (rs *resolver) candidates(key string, flag, prof, def *string) []candidate {
	var out []candidate
	if flag != nil {
		out = append(out, candidate{*flag, "flag"})
	}
	name := envName(key)
	if v := rs.env(name); v != "" {
		out = append(out, candidate{v, "env " + name})
	}
	if prof != nil {
		out = append(out, candidate{*prof, "profile " + rs.identity})
	}
	if def != nil {
		out = append(out, candidate{*def, "default"})
	}
	return out
}

func (rs *resolver) str(key string, flag, prof, def *string, builtin string) string {
	for _, c := range rs.candidates(key, flag, prof, def) {
		rs.out.Source[key] = c.source
		return c.value
	}
	rs.out.Source[key] = "builtin"
	return builtin
}

// num resolves an integer setting. A layer whose text is not a number is
// reported and passed over, so the next layer down still answers.
func (rs *resolver) num(key string, flag, prof, def *int, builtin int) int {
	for _, c := range rs.candidates(key, itoa(flag), itoa(prof), itoa(def)) {
		n, err := strconv.Atoi(c.value)
		if err != nil {
			rs.warn(c, "a number")
			continue
		}
		rs.out.Source[key] = c.source
		return n
	}
	rs.out.Source[key] = "builtin"
	return builtin
}

// boolean resolves a boolean setting, passing over any layer whose text is not
// one of 1, 0, true or false.
func (rs *resolver) boolean(key string, flag, prof, def *bool, builtin bool) bool {
	for _, c := range rs.candidates(key, btoa(flag), btoa(prof), btoa(def)) {
		b, err := parseBool(c.value)
		if err != nil {
			rs.warn(c, "a boolean")
			continue
		}
		rs.out.Source[key] = c.source
		return b
	}
	rs.out.Source[key] = "builtin"
	return builtin
}

func (rs *resolver) warn(c candidate, want string) {
	rs.out.Warnings = append(rs.out.Warnings,
		fmt.Sprintf("%s: %q is not %s, ignored", c.source, c.value, want))
}

func itoa(v *int) *string {
	if v == nil {
		return nil
	}
	s := strconv.Itoa(*v)
	return &s
}

func btoa(v *bool) *string {
	if v == nil {
		return nil
	}
	s := strconv.FormatBool(*v)
	return &s
}

// splitList reads a comma-separated environment value as a list, dropping empty
// entries and surrounding space.
func splitList(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

// parseBool accepts 1, 0, true and false in any case.
func parseBool(s string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "true":
		return true, nil
	case "0", "false":
		return false, nil
	}
	return false, fmt.Errorf("not a boolean: %q", s)
}

// Resolve produces the final value of every setting. flags carries only the
// settings the command line actually named, env reads the environment, file is
// the loaded configuration, and identity selects the profile.
func Resolve(flags Settings, env func(string) string, file File, identity string) Resolved {
	if env == nil {
		env = func(string) string { return "" }
	}
	out := Resolved{Source: make(map[string]string, len(Keys))}
	prof := file.Profiles[identity]
	rs := &resolver{env: env, identity: identity, out: &out}
	def := file.Default

	out.Theme = rs.str(KeyTheme, flags.Theme, prof.Theme, def.Theme, "")
	out.Glyphs = rs.str(KeyGlyphs, flags.Glyphs, prof.Glyphs, def.Glyphs, defaultGlyphs)
	out.Width = rs.num(KeyWidth, flags.Width, prof.Width, def.Width, 0)
	out.Orientation = rs.str(KeyOrientation, flags.Orientation, prof.Orientation, def.Orientation, "")
	out.PaddingX = rs.num(KeyPaddingX, flags.PaddingX, prof.PaddingX, def.PaddingX, defaultPaddingX)
	out.PaddingY = rs.num(KeyPaddingY, flags.PaddingY, prof.PaddingY, def.PaddingY, defaultPaddingY)
	out.SharpEdges = rs.boolean(KeySharpEdges, flags.SharpEdges, prof.SharpEdges, def.SharpEdges, false)
	out.Truecolor = rs.boolean(KeyTruecolor, flags.Truecolor, prof.Truecolor, def.Truecolor, true)
	out.Hyperlinks = rs.boolean(KeyHyperlinks, flags.Hyperlinks, prof.Hyperlinks, def.Hyperlinks, false)
	out.AmbiguousWide = rs.boolean(KeyAmbiguousWide, flags.AmbiguousWide, prof.AmbiguousWide, def.AmbiguousWide, false)

	switch failedEnv := env(envName(KeyFailed)); {
	case flags.Failed != nil:
		out.Failed, out.Source[KeyFailed] = flags.Failed, "flag"
	case failedEnv != "":
		out.Failed, out.Source[KeyFailed] = splitList(failedEnv), "env "+envName(KeyFailed)
	case prof.Failed != nil:
		out.Failed, out.Source[KeyFailed] = prof.Failed, "profile "+identity
	case def.Failed != nil:
		out.Failed, out.Source[KeyFailed] = def.Failed, "default"
	default:
		out.Source[KeyFailed] = "builtin"
	}

	// NO_COLOR suppresses a theme that came from anywhere but the command
	// line: -t is a request made in the same breath as the one that set it.
	if env("NO_COLOR") != "" && out.Theme != "" && out.Source[KeyTheme] != "flag" {
		out.Theme = ""
		out.Source[KeyTheme] = "NO_COLOR"
	}

	return out
}
