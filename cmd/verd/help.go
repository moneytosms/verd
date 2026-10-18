package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"
)

// opt is one row of a help table: a flag, subcommand or example and what it does.
type opt struct{ Name, Desc string }

// cmdDoc is the help page of one command (or subcommand: Name is then "snippet add").
type cmdDoc struct {
	Name     string
	Group    string // root overview section; "" keeps a subcommand off the overview
	Summary  string
	Usage    []string
	About    string // paragraphs separated by a blank line
	Subs     []opt
	Flags    []opt
	Examples []opt // Name is the command line, Desc the comment
	Exit     string
	See      []string
}

// helpGroups orders the overview sections.
var helpGroups = []string{"Get started", "The contest loop", "Customize", "Accounts", "Maintenance"}

var helpDocs = []cmdDoc{
	{
		Name: "init", Group: "Get started", Summary: "write the config, Templates and snippet library",
		Usage: []string{"verd init [--force] [--no-setup]"},
		About: "Writes the commented default config to ~/.config/verd/config.toml, the starter Templates to <snippets_dir>/templates/ and the snippet library to <snippets_dir>/library/. In a terminal it then offers the guided setup.\n\nIt refuses to run when the config already exists, so it never clobbers your settings by accident.",
		Flags: []opt{
			{"--force", "overwrite the config, the Templates and the snippet files with the defaults"},
			{"--no-setup", "do not offer the guided setup afterwards (also implied when stdin is not a terminal)"},
		},
		Examples: []opt{{"verd init", "first-time install"}, {"verd init --force --no-setup", "reset everything to the defaults"}},
		See:      []string{"setup", "config", "snippet"},
	},
	{
		Name: "setup", Group: "Get started", Summary: "guided setup for a new install",
		Usage:    []string{"verd setup"},
		About:    "Asks for your Codeforces handle, Workspace, default language, theme, editor split and submit mode, then writes them into config.toml. Comments and other lines in the file stay. Enter keeps the value shown in brackets, so it is safe to re-run.",
		Examples: []opt{{"verd setup", ""}},
		See:      []string{"init", "config"},
	},
	{
		Name: "doctor", Group: "Get started", Summary: "diagnose config, compilers, editor and terminal",
		Usage: []string{"verd doctor"},
		About: "Checks what verd depends on and prints ok, warn or fail for each: the config file, your handle, the Workspace (exists, creatable, not under /mnt on WSL), the compiler or interpreter of every language, the editor (and Neovim 0.9 or newer), tmux or herdr, truecolor support and $TERM. Run it first when something does not work.",
		Exit:  "0 when nothing failed, 1 otherwise.",
		See:   []string{"setup", "config"},
	},
	{
		Name: "config", Group: "Customize", Summary: "print or change the config",
		Usage: []string{"verd config", "verd config path | keys", "verd config get <key>", "verd config set <key> <value>"},
		About: "With no arguments, prints the effective config: your file merged over the defaults. set edits one key in place, keeps your comments, validates the result and refuses to leave a config that does not load.",
		Subs: []opt{
			{"path", "print the config file path"},
			{"keys", "list every key get and set accept"},
			{"get <key>", "print one value"},
			{"set <key> <value>", "change one value in config.toml"},
		},
		Examples: []opt{{"verd config set theme tokyo-night", ""}, {"verd config get workspace", ""}, {"verd config set snippets_dir ~/dotfiles/verd", "keep Templates and snippets elsewhere"}},
		See:      []string{"setup", "themes", "keys"},
	},
	{
		Name: "test", Group: "The contest loop", Summary: "run Sample and Custom Tests",
		Usage:    []string{"verd test <file>"},
		About:    "Builds the Solution and runs it against the Problem's Sample Tests and your Custom Tests, then prints a Verdict per test. <file> is a Solution inside the Workspace (<workspace>/<contest>/<index>/main.<ext>); the Problem and language come from its path.\n\nWhen the TUI is running, the run shows up in its pane and the result is also printed here. Otherwise it runs headless. Bind it to a key in your editor, for example in Neovim: vim.keymap.set(\"n\", \"<leader>vt\", ...).",
		Exit:     "0 only if every test is AC.",
		Examples: []opt{{"verd test ~/verd/1900/A/main.cpp", ""}},
		See:      []string{"stress", "submit"},
	},
	{
		Name: "stress", Group: "The contest loop", Summary: "search for a counterexample",
		Usage: []string{"verd stress [--iter N] [--time S] [--jobs N] [--batch N] [--multi N] <file>"},
		About: "Runs gen with seeds 1, 2, 3, ... and feeds each Input to your Solution and to brute until they disagree, crash or run out of time. gen and brute are created next to the Solution from your Templates on first use; fill them in and run again. Seeds are deterministic, so the same bug gives the same counterexample every time.\n\nIterations run in parallel. --batch and --multi cut process spawns further for fast programs; both are opt-in and never change the verdicts or the reported seed. See docs/testing.md.",
		Flags: []opt{
			{"--iter N", "stop after N iterations (default 1000)"},
			{"--time S", "stop after S seconds (default 30)"},
			{"--jobs N", "parallel iterations (default: CPU count, at most 16)"},
			{"--batch N", "gen runs once per N seeds as \"gen <seed> <N>\" and prints N cases, each followed by a line \"---\""},
			{"--multi N", "for multi-test problems: Solution and brute read t then t cases, gen prints one case without t, and N cases share one process pair"},
		},
		Exit:     "0 only if no counterexample was found within the limits; 1 for a mismatch, runtime error or time limit; 2 when gen, brute or a build failed.",
		Examples: []opt{{"verd stress main.cpp", "1000 iterations, 30 seconds"}, {"verd stress --iter 100000 --time 120 main.cpp", ""}, {"verd stress --batch 100 --multi 100 main.cpp", "fastest for tiny multi-test programs"}},
		See:      []string{"test", "snippet"},
	},
	{
		Name: "submit", Group: "The contest loop", Summary: "submit the Solution and track the Verdict",
		Usage:    []string{"verd submit <file>"},
		About:    "In browser mode (default) copies the Solution to the clipboard, opens the Problem's submit page and tracks the new Submission. In direct mode (submit_mode = \"direct\", after verd login) it posts the Solution itself. Either way it follows the Submission until a final Verdict arrives and prints each status.",
		Exit:     "0 only if the Submission was Accepted.",
		Examples: []opt{{"verd submit ~/verd/1900/A/main.cpp", ""}},
		See:      []string{"login", "config"},
	},
	{
		Name: "editorial", Group: "The contest loop", Summary: "read a problem's editorial, hints first",
		Usage: []string{"verd editorial [--reveal N | --all] <file|contest/index>"},
		About: "Prints the part of the contest's editorial that is about this problem. Hints, the solution and code are spoilers: they stay hidden until you ask, strictly in order (--reveal 2 opens the first two, --all opens everything). Announcements, video tutorials and discussion streams are listed as links only.\n\nThe editorial link comes from the \"Contest materials\" box of the problem page and the text from the blog post behind it; both are cached, so a second look works offline. Only English editorials are shown. In the TUI press H in the Problem view for the same thing, one hint at a time. It is hidden while a contest session runs on that contest.",
		Flags: []opt{
			{"--reveal N", "open the first N hints and spoilers (default 0)"},
			{"--all", "open every spoiler"},
		},
		Exit:     "0 when an editorial was printed; 1 when none is published, it is Russian only, or the problem is not from Codeforces; 2 on other errors.",
		Examples: []opt{{"verd editorial 1900/A", "text before the first hint"}, {"verd editorial --reveal 1 1900A", "plus the first hint"}, {"verd editorial --all ~/verd/1900/A/main.cpp", "everything"}},
		See:      []string{"test", "snippet"},
	},
	{
		Name: "snippet", Group: "The contest loop", Summary: "print, copy, export and manage library snippets",
		Usage: []string{"verd snippet [--lang L] --list", "verd snippet [--lang L] [--copy] <name>", "verd snippet [--lang L] --export", "verd snippet add|edit|rm|open|path ..."},
		About: "The snippet library holds reusable code (bfs, dijkstra, modpow, prefix_sum, ...) for C++, C and Python. The built-ins ship inside verd; your own files in <snippets_dir>/library/<lang>/ add to them or replace them by name. The first-line comment of a file is its description. In the TUI press i in the Problem view for a picker that copies to the clipboard.\n\nA unique name prefix is enough wherever a name is expected.",
		Subs: []opt{
			{"add <name> [file|-]", "add a snippet from a file or stdin"},
			{"edit <name>", "edit a snippet in your editor"},
			{"rm <name>", "remove your snippet file"},
			{"open", "open the library folder in your editor"},
			{"path", "print the snippets directory"},
		},
		Flags: []opt{
			{"--lang L", "language key: cpp, c, python or your own (default: default_lang)"},
			{"--list", "list names and descriptions"},
			{"--copy", "put the snippet on the clipboard instead of printing it"},
			{"--export", "print the library as VSCode-style JSON for LuaSnip or VSCode"},
		},
		Examples: []opt{
			{"verd snippet --list", "what is available for C++"}, {"verd snippet dijkstra", "print one"}, {"verd snippet --lang python --copy dsu", "to the clipboard"},
			{"verd snippet --export --lang cpp > verd-cpp.json", "for LuaSnip's load_standalone"},
		},
		See: []string{"snippet add", "snippet edit", "snippet rm", "snippet open", "config"},
	},
	{
		Name:    "snippet add",
		Summary: "add a snippet from a file or stdin",
		Usage:   []string{"verd snippet add [--lang L] [--desc D] [--force] <name> [file|-]"},
		About:   "Stores the code as <snippets_dir>/library/<lang>/<name>.<ext>. When the code has no first-line comment, one is written for you: --desc, else the name. With no file and no -, it creates a stub with only the description and opens your editor on it.\n\nIt refuses to replace an existing snippet, built-in ones included, unless you pass --force. Names may contain letters, digits, _ and -.",
		Flags: []opt{
			{"--lang L", "language key (default: default_lang)"},
			{"--desc D", "one-line description"},
			{"--force", "overwrite an existing snippet"},
		},
		Examples: []opt{
			{"verd snippet add --desc \"double it\" twice twice.cpp", "from a file"},
			{"verd snippet add --lang python xone -", "from stdin"},
			{":'<,'>w !verd snippet add mysnip -", "in Neovim: save the selected lines"},
		},
		See: []string{"snippet", "snippet edit"},
	},
	{
		Name:     "snippet edit",
		Summary:  "edit a snippet in your editor",
		Usage:    []string{"verd snippet edit [--lang L] <name>"},
		About:    "Opens the snippet's file in the configured editor. A built-in snippet first gets an editable copy in your library, which then overrides it; verd snippet rm restores the original.",
		Flags:    []opt{{"--lang L", "language key (default: default_lang)"}},
		Examples: []opt{{"verd snippet edit dijkstra", ""}},
		See:      []string{"snippet", "snippet rm"},
	},
	{
		Name:     "snippet rm",
		Summary:  "remove your snippet file",
		Usage:    []string{"verd snippet rm [--lang L] <name>"},
		About:    "Deletes your file for the snippet. If a built-in has the same name, it comes back. Built-ins themselves cannot be removed; override them with snippet edit instead.",
		Flags:    []opt{{"--lang L", "language key (default: default_lang)"}},
		Examples: []opt{{"verd snippet rm twice", ""}},
		See:      []string{"snippet", "snippet edit"},
	},
	{
		Name:    "snippet open",
		Summary: "open the library folder in your editor",
		Usage:   []string{"verd snippet open"},
		About:   "Copies any built-in snippet that is not in your library yet (never overwriting a file), then opens <snippets_dir>/library in the configured editor so you can browse, edit and add files for every language.",
		See:     []string{"snippet", "snippet path"},
	},
	{
		Name:     "snippet path",
		Summary:  "print the snippets directory",
		Usage:    []string{"verd snippet path"},
		About:    "Prints snippets_dir: the folder holding templates/ and library/. Handy in scripts and for cd.",
		Examples: []opt{{"cd \"$(verd snippet path)\"", ""}},
		See:      []string{"snippet", "config"},
	},
	{
		Name: "daily", Group: "The contest loop", Summary: "today's unsolved pick",
		Usage: []string{"verd daily"},
		About: "Prints one unsolved Codeforces Problem near your rating, taken from your weak topics when you have any. It is seeded by the date, so you get the same pick all day. Needs the Problem cache: start verd once to sync it.",
		See:   []string{"sync"},
	},
	{
		Name: "sync", Group: "Accounts", Summary: "pull solved CSES tasks into marks",
		Usage:    []string{"verd sync [cses]"},
		About:    "Fetches the tasks you solved on a provider that has no Submission feed and records them as solved marks. CSES is the only provider today. The first run asks for your CSES username and password to get a session; the password is never stored, only the session cookie.",
		Examples: []opt{{"verd sync cses", ""}},
		See:      []string{"logout"},
	},
	{
		Name: "login", Group: "Accounts", Summary: "save the browser session used by direct submit",
		Usage: []string{"verd login"},
		About: "Reads your logged-in Codeforces Cookie header and User-Agent (two lines on stdin, hidden in a terminal) and saves them in the OS keyring, or in credentials.json with mode 0600 when no keyring is available. Nothing is echoed back. Only needed for submit_mode = \"direct\".",
		See:   []string{"logout", "submit"},
	},
	{
		Name: "logout", Group: "Accounts", Summary: "forget a saved session",
		Usage:    []string{"verd logout [cses]"},
		About:    "Deletes the saved Codeforces session, or with a provider name that provider's session.",
		Examples: []opt{{"verd logout", "Codeforces direct-submit session"}, {"verd logout cses", ""}},
		See:      []string{"login", "sync"},
	},
	{
		Name: "keys", Group: "Customize", Summary: "list and rebind shortcuts",
		Usage: []string{"verd keys [--json]", "verd keys set <ctx.action> <key>...", "verd keys reset <ctx.action>", "verd keys presets | preset <name> | export | import <file|->"},
		About: "Every shortcut in the TUI is an action in a context, written ctx.action (for example problem.stress). With no arguments this lists them all with the default and your current binding. Rebinds are written to config.toml and are checked for conflicts inside one context.",
		Subs: []opt{
			{"list", "list every shortcut (the default)"},
			{"contexts", "list the contexts"},
			{"set <ctx.action> <key>...", "rebind; several keys are allowed"},
			{"reset <ctx.action>", "go back to the default"},
			{"check", "validate the current bindings"},
			{"presets", "list presets: default, vim, arrows, emacs"},
			{"preset <name>", "apply a preset"},
			{"export | import <file|->", "dump or load your bindings as TOML"},
			{"markdown [--write <file>]", "render the key tables as Markdown (docs/keys.md)"},
		},
		Flags:    []opt{{"--json", "machine-readable output for list"}},
		Examples: []opt{{"verd keys set problem.stress X", ""}, {"verd keys preset vim", ""}},
		See:      []string{"config", "themes"},
	},
	{
		Name: "themes", Group: "Customize", Summary: "list themes or print an editable block",
		Usage:    []string{"verd themes [list]", "verd themes show <name>"},
		About:    "Lists the built-in themes. show prints a [themes.my-<name>] block with every color of that theme; paste it into config.toml, change what you like and select it with theme = \"my-<name>\".",
		Examples: []opt{{"verd themes show nord >> ~/.config/verd/config.toml", ""}},
		See:      []string{"config"},
	},
	{
		Name: "update", Group: "Maintenance", Summary: "replace verd with the latest release",
		Usage: []string{"verd update [--check]"},
		About: "Downloads the latest GitHub release for your platform, verifies its checksum and replaces the running binary. A source build (version dev) is refused: update those with go install.",
		Flags: []opt{{"--check", "only report whether a newer release exists"}},
		See:   []string{"version"},
	},
	{
		Name: "completions", Group: "Maintenance", Summary: "print a shell completion script",
		Usage:    []string{"verd completions bash|zsh|fish"},
		About:    "Prints a completion script for commands, config keys, key actions and themes. Source it from your shell's startup file.",
		Examples: []opt{{"source <(verd completions zsh)", "zsh"}, {"verd completions fish > ~/.config/fish/completions/verd.fish", "fish"}},
	},
	{
		Name: "schema", Group: "Maintenance", Summary: "print the JSON Schema of config.toml",
		Usage:    []string{"verd schema"},
		About:    "Prints a JSON Schema for config.toml so editors (taplo, Even Better TOML) can validate and complete it.",
		Examples: []opt{{"verd schema > ~/.config/verd/schema.json", ""}},
	},
	{
		Name: "version", Group: "Maintenance", Summary: "print the version",
		Usage: []string{"verd version", "verd --version"},
	},
	{
		Name: "help", Group: "Maintenance", Summary: "show help for a command",
		Usage:    []string{"verd help [command [subcommand]]", "verd <command> --help"},
		About:    "Prints this overview or the page of one command. Colors and layout are used on a terminal; piped output, NO_COLOR or TERM=dumb give plain text. FORCE_COLOR=1 forces colors.",
		Examples: []opt{{"verd help stress", ""}, {"verd snippet add --help", ""}},
	},
}

// ANSI styles; empty when color is off, so the same code renders both layouts.
type helpStyle struct{ head, name, arg, dim, off string }

func styleFor(color bool) helpStyle {
	if !color {
		return helpStyle{}
	}
	return helpStyle{head: "\x1b[1;36m", name: "\x1b[1;32m", arg: "\x1b[33m", dim: "\x1b[2m", off: "\x1b[0m"}
}

// helpColor says whether help written to out should use colors.
func helpColor(out io.Writer) bool {
	switch {
	case os.Getenv("NO_COLOR") != "":
		return false
	case os.Getenv("FORCE_COLOR") != "" || os.Getenv("CLICOLOR_FORCE") != "":
		return true
	}
	f, ok := out.(*os.File)
	return ok && os.Getenv("TERM") != "dumb" && term.IsTerminal(int(f.Fd()))
}

func (s helpStyle) wrap(style, text string) string {
	if style == "" || text == "" {
		return text
	}
	return style + text + s.off
}

// args highlights <placeholders>, [optionals] and ... in a usage or option line.
func (s helpStyle) args(line string) string {
	if s.arg == "" {
		return line
	}
	var b strings.Builder
	for i := 0; i < len(line); {
		r := line[i]
		if r == '<' || r == '[' {
			closer := byte('>')
			if r == '[' {
				closer = ']'
			}
			if j := strings.IndexByte(line[i:], closer); j > 0 {
				b.WriteString(s.arg + line[i:i+j+1] + s.off)
				i += j + 1
				continue
			}
		}
		b.WriteByte(r)
		i++
	}
	return b.String()
}

// words wraps text to width columns with an indent on every line.
func words(text string, indent, width int) []string {
	var lines []string
	pad := strings.Repeat(" ", indent)
	for _, para := range strings.Split(text, "\n\n") {
		cur := ""
		for _, w := range strings.Fields(para) {
			if cur != "" && indent+len(cur)+1+len(w) > width {
				lines = append(lines, pad+cur)
				cur = ""
			}
			if cur != "" {
				cur += " "
			}
			cur += w
		}
		if cur != "" {
			lines = append(lines, pad+cur)
		}
		lines = append(lines, "")
	}
	return lines[:len(lines)-1]
}

func table(w io.Writer, s helpStyle, rows []opt, nameStyle string) {
	col := 0
	for _, r := range rows {
		col = max(col, len(r.Name))
	}
	col = min(col, 38)
	for _, r := range rows {
		name := s.wrap(nameStyle, r.Name)
		if nameStyle == "" {
			name = s.args(r.Name)
		}
		if r.Desc == "" {
			fmt.Fprintf(w, "  %s\n", name)
			continue
		}
		pad := col - len(r.Name)
		if pad < 0 { // long name: the description goes on its own line
			fmt.Fprintf(w, "  %s\n  %s%s\n", name, strings.Repeat(" ", col+2), r.Desc)
			continue
		}
		fmt.Fprintf(w, "  %s%s  %s\n", name, strings.Repeat(" ", pad), r.Desc)
	}
}

func section(w io.Writer, s helpStyle, title string) {
	fmt.Fprintf(w, "\n%s\n", s.wrap(s.head, title))
}

// renderDoc writes one help page.
func renderDoc(w io.Writer, d cmdDoc, color bool) {
	s := styleFor(color)
	fmt.Fprintf(w, "%s  %s\n", s.wrap(s.name, "verd "+d.Name), s.wrap(s.dim, d.Summary))
	section(w, s, "USAGE")
	for _, u := range d.Usage {
		rest := strings.TrimPrefix(u, "verd ")
		fmt.Fprintf(w, "  %s %s\n", s.wrap(s.dim, "verd"), s.args(rest))
	}
	if d.About != "" {
		section(w, s, "DESCRIPTION")
		for _, l := range words(d.About, 2, 78) {
			fmt.Fprintln(w, l)
		}
	}
	if len(d.Subs) > 0 {
		section(w, s, "COMMANDS")
		table(w, s, d.Subs, s.name)
	}
	if len(d.Flags) > 0 {
		section(w, s, "OPTIONS")
		table(w, s, d.Flags, s.name)
	}
	if len(d.Examples) > 0 {
		section(w, s, "EXAMPLES")
		col := 0
		for _, e := range d.Examples {
			if len(e.Name) <= 52 {
				col = max(col, len(e.Name))
			}
		}
		for _, e := range d.Examples {
			if e.Desc == "" {
				fmt.Fprintf(w, "  %s\n", e.Name)
			} else {
				fmt.Fprintf(w, "  %s%s  %s\n", e.Name, strings.Repeat(" ", max(0, col-len(e.Name))), s.wrap(s.dim, "# "+e.Desc))
			}
		}
	}
	if d.Exit != "" {
		section(w, s, "EXIT STATUS")
		for _, l := range words(d.Exit, 2, 78) {
			fmt.Fprintln(w, l)
		}
	}
	if len(d.See) > 0 {
		section(w, s, "SEE ALSO")
		for _, r := range d.See {
			fmt.Fprintf(w, "  %s %s\n", s.wrap(s.dim, "verd help"), s.wrap(s.name, r))
		}
	}
}

// renderRoot writes the overview: every command by group.
func renderRoot(w io.Writer, color bool) {
	s := styleFor(color)
	fmt.Fprintf(w, "%s  %s\n", s.wrap(s.name, "verd"), s.wrap(s.dim, "terminal Codeforces + CSES: pick, read, edit, test, stress, submit"))
	section(w, s, "USAGE")
	fmt.Fprintf(w, "  %s %s\n", s.wrap(s.dim, "verd"), s.args("[--here] [command]"))
	fmt.Fprintf(w, "  %s\n", s.wrap(s.dim, "no command: open the TUI"))
	for _, g := range helpGroups {
		var rows []opt
		for _, d := range helpDocs {
			if d.Group == g {
				rows = append(rows, opt{d.Name, d.Summary})
			}
		}
		if len(rows) > 0 {
			section(w, s, strings.ToUpper(g))
			table(w, s, rows, s.name)
		}
	}
	section(w, s, "OPTIONS")
	table(w, s, []opt{
		{"--here", "use the current directory as the Workspace for this run"},
		{"-h, --help", "show help (also after a command: verd stress --help)"},
		{"--version", "print the version"},
	}, s.name)
	section(w, s, "MORE")
	fmt.Fprintf(w, "  %s for one command, docs in docs/ and at https://moneytosms.github.io/verd\n", s.wrap(s.name, "verd help <command>"))
}

// findDoc resolves "snippet add" style topics, trying two words and then one.
func findDoc(topic []string) (cmdDoc, bool) {
	for n := min(len(topic), 2); n >= 1; n-- {
		name := strings.Join(topic[:n], " ")
		for _, d := range helpDocs {
			if d.Name == name {
				return d, true
			}
		}
	}
	return cmdDoc{}, false
}

// showHelp prints the overview (no topic) or one page; unknown topics are an error.
func showHelp(out io.Writer, topic []string) error {
	color := helpColor(out)
	if len(topic) == 0 {
		renderRoot(out, color)
		return nil
	}
	d, ok := findDoc(topic)
	if !ok {
		var names []string
		for _, d := range helpDocs {
			if d.Group != "" {
				names = append(names, d.Name)
			}
		}
		return &exitError{2, fmt.Sprintf("no help for %q (commands: %s)", strings.Join(topic, " "), strings.Join(names, ", "))}
	}
	renderDoc(out, d, color)
	return nil
}

// wantsHelp reports whether args ask for help and returns the topic words.
func wantsHelp(args []string) (topic []string, ok bool) {
	if len(args) == 0 || args[0] == "__complete" {
		return nil, false
	}
	if args[0] == "help" {
		return args[1:], true
	}
	for _, a := range args {
		if a == "-h" || a == "--help" {
			ok = true
		} else if !strings.HasPrefix(a, "-") && len(topic) < 2 {
			topic = append(topic, a)
		}
	}
	if !ok {
		return nil, false
	}
	if len(topic) > 0 && topic[0] != args[0] { // verd --help stress
		topic = topic[:0]
	}
	return topic, true
}
