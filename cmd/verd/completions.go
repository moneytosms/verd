package main

import (
	"fmt"
	"io"
)

// completionsCmd handles `verd completions bash|zsh|fish` and the hidden `verd __complete <what>`.
func completionsCmd(out io.Writer, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: verd completions bash|zsh|fish")
	}

	shell := args[0]
	if shell == "bash" {
		fmt.Fprint(out, bashCompletionScript)
		return nil
	}
	if shell == "zsh" {
		fmt.Fprint(out, zshCompletionScript)
		return nil
	}
	if shell == "fish" {
		fmt.Fprint(out, fishCompletionScript)
		return nil
	}
	return fmt.Errorf("unknown shell %q (bash, zsh, fish)", shell)
}

// completeCmd handles `verd __complete config-keys|key-actions|themes`.
// For now, returns empty completions (full implementation would use internals).
func completeCmd(out io.Writer, what string) error {
	switch what {
	case "config-keys":
		// Static list of settable keys
		for _, k := range []string{"autotest", "background", "default_lang", "editor", "embed_focus_key", "embed_ratio", "embed_side", "float_eps", "handle", "source_cf", "source_cses", "split", "submit_mode", "theme", "time_multiplier", "workspace"} {
			fmt.Fprintln(out, k)
		}
	case "key-actions":
		// Static list of key actions
		fmt.Fprintln(out, "common.help")
		fmt.Fprintln(out, "common.dismiss_toast")
		// ... (would be many more in full version)
	case "themes":
		// Static list of themes
		for _, n := range []string{"terminal", "tokyo-night", "dracula", "catppuccin", "gruvbox", "nord", "one-dark", "solarized"} {
			fmt.Fprintln(out, n)
		}
	default:
		return fmt.Errorf("unknown completion type %q (config-keys, key-actions, themes)", what)
	}
	return nil
}

// Bash completion script: calls __complete for dynamic options.
const bashCompletionScript = `# verd completion for bash
# Install: eval "$(verd completions bash)"
# Or append to ~/.bashrc: complete -o nosort -C 'verd __complete_bash' verd

_verd_complete() {
    local cur prev words cword
    cur="${COMP_WORDS[COMP_CWORD]}"
    prev="${COMP_WORDS[COMP_CWORD-1]}"
    words=("${COMP_WORDS[@]}")
    cword=$COMP_CWORD

    case "${words[1]}" in
        config)
            if [[ "$cword" -eq 2 ]]; then
                echo -e "path\nkeys\nget\nset"
            elif [[ "$cword" -eq 3 && "${words[2]}" == "set" ]]; then
                verd __complete config-keys
            fi
            ;;
        keys)
            if [[ "$cword" -eq 2 ]]; then
                echo -e "--json\nlist\ncontexts\ncheck\nset\nreset"
            elif [[ "$cword" -eq 3 && "${words[2]}" == "set" ]]; then
                verd __complete key-actions
            fi
            ;;
        themes)
            if [[ "$cword" -eq 2 ]]; then
                echo -e "list\nshow"
            elif [[ "$cword" -eq 3 && "${words[2]}" == "show" ]]; then
                verd __complete themes
            fi
            ;;
        *)
            if [[ "$cword" -eq 1 ]]; then
                echo -e "init\nsetup\nconfig\ntest\nstress\nsubmit\nupdate\ndaily\nsync\nlogin\nlogout\nkeys\nthemes\ndoctor\ncompletions\nversion"
            fi
            ;;
    esac
}

complete -o nosort -F _verd_complete verd
`

// Zsh completion script.
const zshCompletionScript = `#compdef verd

_verd() {
    local state
    _arguments \
        '1: :->cmd' \
        '2: :->subcmd' \
        '3: :->arg'

    case "$state" in
        cmd)
            local commands=(
                "init:write config and Templates"
                "setup:guided setup"
                "config:print or change config"
                "test:run tests"
                "stress:stress test"
                "submit:submit solution"
                "update:update verd"
                "daily:daily pick"
                "sync:sync solved marks"
                "login:save browser session"
                "logout:delete browser session"
                "keys:list or rebind shortcuts"
                "themes:list or show themes"
                "doctor:diagnostics"
                "completions:shell completion"
                "version:print version"
            )
            _describe 'command' commands
            ;;
        subcmd)
            case "${words[2]}" in
                config)
                    local subcmds=(
                        "path:config file path"
                        "keys:settable keys"
                        "get:read one key"
                        "set:change one key"
                    )
                    _describe 'config subcommand' subcmds
                    ;;
                keys)
                    local subcmds=(
                        "list:list shortcuts"
                        "contexts:list contexts"
                        "check:validate config"
                        "set:rebind shortcut"
                        "reset:reset to default"
                    )
                    _describe 'keys subcommand' subcmds
                    ;;
                themes)
                    local subcmds=(
                        "list:list themes"
                        "show:show theme config"
                    )
                    _describe 'themes subcommand' subcmds
                    ;;
                completions)
                    local subcmds=(
                        "bash:bash completion"
                        "zsh:zsh completion"
                        "fish:fish completion"
                    )
                    _describe 'shell' subcmds
                    ;;
            esac
            ;;
        arg)
            case "${words[2]}" in
                config)
                    if [[ "${words[3]}" == "set" ]]; then
                        compadd $(verd __complete config-keys)
                    fi
                    ;;
                keys)
                    if [[ "${words[3]}" == "set" ]]; then
                        compadd $(verd __complete key-actions)
                    fi
                    ;;
                themes)
                    if [[ "${words[3]}" == "show" ]]; then
                        compadd $(verd __complete themes)
                    fi
                    ;;
            esac
            ;;
    esac
}

_verd
`

// Fish completion script.
const fishCompletionScript = `# verd completion for fish
# Install: verd completions fish | source

complete -c verd -n "__fish_seen_subcommand_from" -f
complete -c verd -n "__fish_use_subcommand_only" -f

# Main commands
complete -c verd -n "__fish_use_subcommand_only" -a "init" -d "write config and Templates"
complete -c verd -n "__fish_use_subcommand_only" -a "setup" -d "guided setup"
complete -c verd -n "__fish_use_subcommand_only" -a "config" -d "print or change config"
complete -c verd -n "__fish_use_subcommand_only" -a "test" -d "run tests"
complete -c verd -n "__fish_use_subcommand_only" -a "stress" -d "stress test"
complete -c verd -n "__fish_use_subcommand_only" -a "submit" -d "submit solution"
complete -c verd -n "__fish_use_subcommand_only" -a "update" -d "update verd"
complete -c verd -n "__fish_use_subcommand_only" -a "daily" -d "daily pick"
complete -c verd -n "__fish_use_subcommand_only" -a "sync" -d "sync solved marks"
complete -c verd -n "__fish_use_subcommand_only" -a "login" -d "save browser session"
complete -c verd -n "__fish_use_subcommand_only" -a "logout" -d "delete browser session"
complete -c verd -n "__fish_use_subcommand_only" -a "keys" -d "list or rebind shortcuts"
complete -c verd -n "__fish_use_subcommand_only" -a "themes" -d "list or show themes"
complete -c verd -n "__fish_use_subcommand_only" -a "doctor" -d "diagnostics"
complete -c verd -n "__fish_use_subcommand_only" -a "completions" -d "shell completion"
complete -c verd -n "__fish_use_subcommand_only" -a "version" -d "print version"

# config subcommands
complete -c verd -n "__fish_seen_subcommand_from config" -f
complete -c verd -n "__fish_seen_subcommand_from config" -a "path" -d "config file path"
complete -c verd -n "__fish_seen_subcommand_from config" -a "keys" -d "settable keys"
complete -c verd -n "__fish_seen_subcommand_from config" -a "get" -d "read one key"
complete -c verd -n "__fish_seen_subcommand_from config" -a "set" -d "change one key"
complete -c verd -n "__fish_seen_subcommand_from config; __fish_seen_subcommand_from set" -a "(verd __complete config-keys)" -d "key"

# keys subcommands
complete -c verd -n "__fish_seen_subcommand_from keys" -f
complete -c verd -n "__fish_seen_subcommand_from keys" -a "list" -d "list shortcuts"
complete -c verd -n "__fish_seen_subcommand_from keys" -a "contexts" -d "list contexts"
complete -c verd -n "__fish_seen_subcommand_from keys" -a "check" -d "validate config"
complete -c verd -n "__fish_seen_subcommand_from keys" -a "set" -d "rebind shortcut"
complete -c verd -n "__fish_seen_subcommand_from keys" -a "reset" -d "reset to default"
complete -c verd -n "__fish_seen_subcommand_from keys; __fish_seen_subcommand_from set" -a "(verd __complete key-actions)" -d "action"

# themes subcommands
complete -c verd -n "__fish_seen_subcommand_from themes" -f
complete -c verd -n "__fish_seen_subcommand_from themes" -a "list" -d "list themes"
complete -c verd -n "__fish_seen_subcommand_from themes" -a "show" -d "show theme config"
complete -c verd -n "__fish_seen_subcommand_from themes; __fish_seen_subcommand_from show" -a "(verd __complete themes)" -d "theme"

# completions subcommands
complete -c verd -n "__fish_seen_subcommand_from completions" -f
complete -c verd -n "__fish_seen_subcommand_from completions" -a "bash" -d "bash completion"
complete -c verd -n "__fish_seen_subcommand_from completions" -a "zsh" -d "zsh completion"
complete -c verd -n "__fish_seen_subcommand_from completions" -a "fish" -d "fish completion"
`
