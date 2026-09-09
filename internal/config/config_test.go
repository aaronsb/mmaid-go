package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func s(v string) *string { return &v }
func i(v int) *int       { return &v }
func b(v bool) *bool     { return &v }

// envMap turns a map into the lookup Resolve takes.
func envMap(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestResolvePrecedence(t *testing.T) {
	file := File{
		Default:  Settings{Theme: s("slate"), Width: i(90)},
		Profiles: map[string]Settings{"WezTerm": {Theme: s("blueprint"), Width: i(100)}},
	}
	env := envMap(map[string]string{"MMAID_THEME": "amber", "MMAID_WIDTH": "110"})

	cases := []struct {
		name       string
		flags      Settings
		env        func(string) string
		file       File
		identity   string
		wantTheme  string
		wantSource string
		wantWidth  int
	}{
		{"flag wins", Settings{Theme: s("mono"), Width: i(120)}, env, file, "WezTerm", "mono", "flag", 120},
		{"env beats profile", Settings{}, env, file, "WezTerm", "amber", "env MMAID_THEME", 110},
		{"profile beats default", Settings{}, nil, file, "WezTerm", "blueprint", "profile WezTerm", 100},
		{"default when no profile", Settings{}, nil, file, "xterm", "slate", "default", 90},
		{"builtin when file empty", Settings{}, nil, File{}, "xterm", "", "builtin", 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Resolve(tc.flags, tc.env, tc.file, tc.identity)
			if got.Theme != tc.wantTheme {
				t.Errorf("theme = %q, want %q", got.Theme, tc.wantTheme)
			}
			if got.Source[KeyTheme] != tc.wantSource {
				t.Errorf("theme source = %q, want %q", got.Source[KeyTheme], tc.wantSource)
			}
			if got.Width != tc.wantWidth {
				t.Errorf("width = %d, want %d", got.Width, tc.wantWidth)
			}
		})
	}
}

func TestResolveBuiltins(t *testing.T) {
	got := Resolve(Settings{}, nil, File{}, "")
	if got.Glyphs != "unicode" || got.PaddingX != 4 || got.PaddingY != 2 {
		t.Errorf("builtins = %q %d %d, want unicode 4 2", got.Glyphs, got.PaddingX, got.PaddingY)
	}
	if !got.Truecolor {
		t.Error("truecolor should default true")
	}
	if got.Hyperlinks || got.SharpEdges || got.AmbiguousWide {
		t.Error("hyperlinks, sharp_edges and ambiguous_wide should default false")
	}
	for _, k := range Keys {
		if got.Source[k] != "builtin" {
			t.Errorf("%s source = %q, want builtin", k, got.Source[k])
		}
	}
}

func TestResolveBooleanForms(t *testing.T) {
	for _, form := range []struct {
		text string
		want bool
	}{{"1", true}, {"true", true}, {"TRUE", true}, {"0", false}, {"false", false}, {"False", false}} {
		got := Resolve(Settings{}, envMap(map[string]string{"MMAID_HYPERLINKS": form.text}), File{}, "")
		if got.Hyperlinks != form.want {
			t.Errorf("MMAID_HYPERLINKS=%q gave %v, want %v", form.text, got.Hyperlinks, form.want)
		}
		if got.Source[KeyHyperlinks] != "env MMAID_HYPERLINKS" {
			t.Errorf("source = %q", got.Source[KeyHyperlinks])
		}
	}
}

func TestResolveFallsThroughAnUnparseableEnv(t *testing.T) {
	file := File{
		Default:  Settings{Width: i(50)},
		Profiles: map[string]Settings{"WezTerm": {Truecolor: b(false)}},
	}
	env := envMap(map[string]string{"MMAID_WIDTH": "wide", "MMAID_TRUECOLOR": "maybe"})

	got := Resolve(Settings{}, env, file, "WezTerm")
	if got.Width != 50 || got.Source[KeyWidth] != "default" {
		t.Errorf("width = %d from %q, want 50 from default", got.Width, got.Source[KeyWidth])
	}
	if got.Truecolor || got.Source[KeyTruecolor] != "profile WezTerm" {
		t.Errorf("truecolor = %v from %q, want false from the profile", got.Truecolor, got.Source[KeyTruecolor])
	}
	if len(got.Warnings) != 2 {
		t.Errorf("warnings = %v, want two", got.Warnings)
	}

	// With no layer below it, the built-in default still answers.
	got = Resolve(Settings{}, env, File{}, "")
	if got.Width != 0 || got.Source[KeyWidth] != "builtin" {
		t.Errorf("width = %d from %q, want 0 from builtin", got.Width, got.Source[KeyWidth])
	}
	if !got.Truecolor {
		t.Error("with nothing below it the builtin should stand")
	}
}

func TestResolveFailedFromEnv(t *testing.T) {
	file := File{Profiles: map[string]Settings{"WezTerm": {Failed: []string{"octants"}}}}
	env := envMap(map[string]string{"MMAID_FAILED": "box-heavy, sextants ,"})

	got := Resolve(Settings{}, env, file, "WezTerm")
	if strings.Join(got.Failed, "|") != "box-heavy|sextants" {
		t.Errorf("failed = %v, want the environment's list", got.Failed)
	}
	if got.Source[KeyFailed] != "env MMAID_FAILED" {
		t.Errorf("failed source = %q", got.Source[KeyFailed])
	}

	got = Resolve(Settings{Failed: []string{"braille"}}, env, file, "WezTerm")
	if got.Source[KeyFailed] != "flag" {
		t.Errorf("a flag should outrank MMAID_FAILED, got %q", got.Source[KeyFailed])
	}
}

func TestResolveEveryEnvName(t *testing.T) {
	env := envMap(map[string]string{
		"MMAID_THEME":          "amber",
		"MMAID_GLYPHS":         "ascii",
		"MMAID_WIDTH":          "77",
		"MMAID_ORIENTATION":    "LR",
		"MMAID_PADDING_X":      "6",
		"MMAID_PADDING_Y":      "3",
		"MMAID_SHARP_EDGES":    "1",
		"MMAID_TRUECOLOR":      "0",
		"MMAID_HYPERLINKS":     "1",
		"MMAID_AMBIGUOUS_WIDE": "1",
	})
	got := Resolve(Settings{}, env, File{}, "")
	if got.Theme != "amber" || got.Glyphs != "ascii" || got.Width != 77 || got.Orientation != "LR" {
		t.Errorf("strings and width resolved wrong: %+v", got)
	}
	if got.PaddingX != 6 || got.PaddingY != 3 {
		t.Errorf("padding = %d,%d", got.PaddingX, got.PaddingY)
	}
	if !got.SharpEdges || got.Truecolor || !got.Hyperlinks || !got.AmbiguousWide {
		t.Errorf("booleans resolved wrong: %+v", got)
	}
	for _, k := range Keys {
		if k == KeyFailed {
			continue
		}
		if want := "env " + envName(k); got.Source[k] != want {
			t.Errorf("%s source = %q, want %q", k, got.Source[k], want)
		}
	}
}

func TestResolveFailedList(t *testing.T) {
	file := File{
		Default:  Settings{Failed: []string{"octants"}},
		Profiles: map[string]Settings{"Apple_Terminal": {Failed: []string{"box-heavy", "octants"}}},
	}
	got := Resolve(Settings{}, nil, file, "Apple_Terminal")
	if strings.Join(got.Failed, ",") != "box-heavy,octants" {
		t.Errorf("failed = %v", got.Failed)
	}
	if got.Source[KeyFailed] != "profile Apple_Terminal" {
		t.Errorf("failed source = %q", got.Source[KeyFailed])
	}
	if got = Resolve(Settings{}, nil, file, "xterm"); got.Source[KeyFailed] != "default" {
		t.Errorf("failed source = %q, want default", got.Source[KeyFailed])
	}
}

func TestNoColorSuppressesInheritedTheme(t *testing.T) {
	file := File{Profiles: map[string]Settings{"WezTerm": {Theme: s("blueprint")}}}

	got := Resolve(Settings{}, envMap(map[string]string{"NO_COLOR": "1"}), file, "WezTerm")
	if got.Theme != "" {
		t.Errorf("theme = %q, want none under NO_COLOR", got.Theme)
	}
	if got.Source[KeyTheme] != "NO_COLOR" {
		t.Errorf("theme source = %q, want NO_COLOR", got.Source[KeyTheme])
	}

	got = Resolve(Settings{}, envMap(map[string]string{"NO_COLOR": "1", "MMAID_THEME": "amber"}), file, "WezTerm")
	if got.Theme != "" || got.Source[KeyTheme] != "NO_COLOR" {
		t.Errorf("NO_COLOR should suppress an environment theme, got %q from %q", got.Theme, got.Source[KeyTheme])
	}

	got = Resolve(Settings{Theme: s("mono")}, envMap(map[string]string{"NO_COLOR": "1"}), file, "WezTerm")
	if got.Theme != "mono" || got.Source[KeyTheme] != "flag" {
		t.Errorf("-t should survive NO_COLOR, got %q from %q", got.Theme, got.Source[KeyTheme])
	}
}

func TestLoadMissingFileIsEmpty(t *testing.T) {
	f, err := Load(filepath.Join(t.TempDir(), "absent.json"))
	if err != nil {
		t.Fatalf("missing file: %v", err)
	}
	if f.Default.Theme != nil || len(f.Profiles) != 0 {
		t.Errorf("missing file gave %+v", f)
	}
}

func TestLoadTreatsAWrongPathKindAsAbsent(t *testing.T) {
	dir := t.TempDir()

	// config.json is a directory.
	asDir := filepath.Join(dir, "config.json")
	if err := os.Mkdir(asDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if f, err := Load(asDir); err != nil || f.Default.Theme != nil {
		t.Errorf("a directory in the file's place gave %+v, %v", f, err)
	}

	// mmaid is a regular file, so the path runs through it.
	asFile := filepath.Join(dir, "mmaid")
	if err := os.WriteFile(asFile, []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	if f, err := Load(filepath.Join(asFile, "config.json")); err != nil || f.Default.Theme != nil {
		t.Errorf("a file in the directory's place gave %+v, %v", f, err)
	}

	if f, err := Load(""); err != nil || f.Default.Theme != nil {
		t.Errorf("an empty path gave %+v, %v", f, err)
	}
}

func TestLoadMalformedNamesThePath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(path)
	if err == nil {
		t.Fatal("malformed JSON should be an error")
	}
	if !strings.Contains(err.Error(), path) {
		t.Errorf("error %q does not name %q", err, path)
	}
}

func TestLoadReadsTheADRShape(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	body := `{
	  "default": { "theme": "blueprint", "glyphs": "unicode", "width": 0 },
	  "profiles": {
	    "WezTerm": { "truecolor": true, "hyperlinks": true },
	    "Apple_Terminal": { "truecolor": false, "failed": ["box-heavy", "octants"] }
	  }
	}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if f.Default.Theme == nil || *f.Default.Theme != "blueprint" {
		t.Errorf("default theme = %v", f.Default.Theme)
	}
	if f.Default.Width == nil || *f.Default.Width != 0 {
		t.Error("an explicit zero width must stay distinguishable from unset")
	}
	got := Resolve(Settings{}, nil, f, "WezTerm")
	if !got.Hyperlinks || got.Theme != "blueprint" {
		t.Errorf("resolved %+v", got)
	}
	if got = Resolve(Settings{}, nil, f, "Apple_Terminal"); got.Truecolor {
		t.Error("Apple_Terminal profile turns truecolor off")
	}
}

func TestPathHonoursXDG(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/tmp/xdg")
	if got, want := Path(), filepath.Join("/tmp/xdg", "mmaid", "config.json"); got != want {
		t.Errorf("Path() = %q, want %q", got, want)
	}
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", "/home/someone")
	if got, want := Path(), filepath.Join("/home/someone", ".config", "mmaid", "config.json"); got != want {
		t.Errorf("Path() = %q, want %q", got, want)
	}

	// With nowhere to look there is no file, rather than one under the
	// working directory.
	t.Setenv("HOME", "")
	if got := Path(); got != "" {
		t.Errorf("Path() = %q, want empty with no HOME", got)
	}
}

func TestTerminalIdentityPrefersTermProgram(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("TERM_PROGRAM", "WezTerm")
	if got := TerminalIdentity(); got != "WezTerm" {
		t.Errorf("identity = %q, want WezTerm", got)
	}
	t.Setenv("TERM_PROGRAM", "")
	if got := TerminalIdentity(); got != "xterm-256color" {
		t.Errorf("identity = %q, want xterm-256color", got)
	}
}

func TestResolvedValueFormatting(t *testing.T) {
	r := Resolve(Settings{Width: i(0), Failed: []string{}}, nil, File{}, "")
	if got := r.Value(KeyWidth); got != "auto" {
		t.Errorf("width value = %q, want auto", got)
	}
	if got := r.Value(KeyTheme); got != "(none)" {
		t.Errorf("theme value = %q", got)
	}
	if got := r.Value(KeyFailed); got != "(none)" {
		t.Errorf("failed value = %q", got)
	}
}
