package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/moneytosms/verd/internal/creds"
	"golang.org/x/term"
)

// loginCmd saves the browser session direct submit uses. Cookie and User-Agent are read from
// in: two lines (hidden when in is a terminal). Nothing is echoed back.
func loginCmd(out io.Writer, in *os.File, store creds.Store) error {
	tty := term.IsTerminal(int(in.Fd()))
	var cookie, ua string
	if tty {
		fmt.Fprintln(out, "In a logged-in Chrome: DevTools > Network > reload codeforces.com/problemset/submit > the 'submit' request > Request Headers.")
		fmt.Fprint(out, "cookie header (hidden): ")
		b, err := term.ReadPassword(int(in.Fd()))
		fmt.Fprintln(out)
		if err != nil {
			return err
		}
		cookie = string(b)
		fmt.Fprint(out, "user-agent: ")
	}
	r := bufio.NewReader(in)
	if !tty {
		cookie, _ = r.ReadString('\n')
	}
	ua, _ = r.ReadString('\n')
	cookie, ua = strings.TrimSpace(cookie), strings.TrimSpace(ua)
	if cookie == "" || ua == "" {
		return &exitError{2, "need a cookie header and a user-agent (two lines on stdin)"}
	}
	where, warn, err := store.Save(creds.Creds{Cookie: cookie, UserAgent: ua})
	if err != nil {
		return err
	}
	if warn != "" {
		fmt.Fprintln(os.Stderr, "verd: warning:", warn)
	}
	fmt.Fprintf(out, "saved to %s. Set submit_mode = \"direct\" in the config to use it.\n", where)
	return nil
}

func logoutCmd(out io.Writer, store creds.Store) error {
	if err := store.Delete(); err != nil {
		return err
	}
	fmt.Fprintln(out, "credentials deleted")
	return nil
}
