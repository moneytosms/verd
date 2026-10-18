package main

import (
	"bytes"
	"strings"
	"testing"
)

// every command run() dispatches on
var allCommands = []string{"init", "setup", "doctor", "config", "test", "stress", "submit", "snippet", "editorial", "daily", "sync", "login", "logout", "keys", "themes", "update", "uninstall", "completions", "schema", "version", "help"}

func TestEveryCommandHasDetailedHelp(t *testing.T) {
	for _, name := range allCommands {
		d, ok := findDoc([]string{name})
		if !ok {
			t.Errorf("no help page for %q", name)
			continue
		}
		if d.Group == "" || d.Summary == "" || len(d.Usage) == 0 {
			t.Errorf("%s: group, summary and usage are required: %+v", name, d)
		}
		if name != "version" && len(d.About) < 40 {
			t.Errorf("%s: description too thin", name)
		}
	}
	for _, name := range []string{"snippet add", "snippet edit", "snippet rm", "snippet open", "snippet path"} {
		if _, ok := findDoc(strings.Fields(name)); !ok {
			t.Errorf("no help page for %q", name)
		}
	}
	for _, d := range helpDocs {
		for _, see := range d.See {
			if _, ok := findDoc(strings.Fields(see)); !ok {
				t.Errorf("%s: see also %q has no page", d.Name, see)
			}
		}
	}
}

func TestHelpPlainAndColor(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	t.Setenv("FORCE_COLOR", "")
	var plain bytes.Buffer
	if err := run([]string{"stress", "--help"}, &plain); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"verd stress", "USAGE", "DESCRIPTION", "OPTIONS", "--jobs N", "EXAMPLES", "EXIT STATUS", "SEE ALSO"} {
		if !strings.Contains(plain.String(), want) {
			t.Errorf("plain help missing %q:\n%s", want, plain.String())
		}
	}
	if strings.Contains(plain.String(), "\x1b") {
		t.Error("a pipe must get plain text")
	}

	t.Setenv("FORCE_COLOR", "1")
	var rich bytes.Buffer
	if err := run([]string{"help", "stress"}, &rich); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(rich.String(), "\x1b[1;36mUSAGE") || !strings.Contains(rich.String(), "\x1b[33m<file>") {
		t.Errorf("colored help should style headings and placeholders:\n%q", rich.String())
	}
	t.Setenv("NO_COLOR", "1")
	var off bytes.Buffer
	run([]string{"help", "stress"}, &off)
	if strings.Contains(off.String(), "\x1b") || off.String() != plain.String() {
		t.Error("NO_COLOR wins over FORCE_COLOR and gives the plain layout")
	}
}

func TestHelpRouting(t *testing.T) {
	var out bytes.Buffer
	for args, want := range map[string]string{
		"snippet add --help": "verd snippet add",
		"help snippet rm":    "verd snippet rm",
		"--help":             "THE CONTEST LOOP",
		"help":               "GET STARTED",
		"keys -h":            "verd keys",
		"snippet --help":     "snippet open",
	} {
		out.Reset()
		if err := run(strings.Fields(args), &out); err != nil || !strings.Contains(out.String(), want) {
			t.Errorf("verd %s: %v, want %q in:\n%s", args, err, want, out.String())
		}
	}
	if err := run([]string{"help", "nonsense"}, &out); err == nil || !strings.Contains(err.Error(), "commands:") {
		t.Errorf("unknown topic should list commands: %v", err)
	}
}

func TestUsageHint(t *testing.T) {
	if h := usageHint([]string{"snippet", "add", "x"}, "usage: verd snippet add ..."); !strings.Contains(h, "verd help snippet add") {
		t.Errorf("hint: %q", h)
	}
	if h := usageHint([]string{"stress"}, "usage: verd stress"); !strings.Contains(h, "verd help stress") {
		t.Errorf("hint: %q", h)
	}
	if usageHint([]string{"stress"}, "gen failed") != "" || usageHint(nil, "usage:") != "" {
		t.Error("no hint without a usage error and a command")
	}
}
