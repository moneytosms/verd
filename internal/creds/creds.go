// Package creds stores the browser session `verd login` captures: the OS keyring when it works,
// else a 0600 file.
package creds

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/zalando/go-keyring"
)

const (
	service = "verd"
	user    = "codeforces"
)

type Creds struct {
	Cookie    string `json:"cookie"`
	UserAgent string `json:"user_agent"`
}

// ErrNone means nobody is logged in.
var ErrNone = errors.New("not logged in: run `verd login`")

// Store keeps Creds in the keyring, falling back to File. Account names the keyring entry
// ("codeforces" when empty), so each provider's session is stored separately.
type Store struct{ File, Account string }

func (s Store) account() string {
	if s.Account == "" {
		return user
	}
	return s.Account
}

// Save returns where the credentials went; warn is set when the file fallback was used.
func (s Store) Save(c Creds) (where, warn string, err error) {
	b, _ := json.Marshal(c)
	kerr := keyring.Set(service, s.account(), string(b))
	if kerr == nil {
		os.Remove(s.File) // never leave a stale copy behind
		return "OS keyring", "", nil
	}
	if err := os.MkdirAll(filepath.Dir(s.File), 0o700); err != nil {
		return "", "", err
	}
	if err := os.WriteFile(s.File, b, 0o600); err != nil {
		return "", "", err
	}
	os.Chmod(s.File, 0o600) // WriteFile keeps the mode of an existing file
	return s.File, "keyring unavailable (" + kerr.Error() + "): stored in a plain 0600 file", nil
}

func (s Store) Load() (Creds, error) {
	var c Creds
	raw, err := keyring.Get(service, s.account())
	if err != nil {
		b, ferr := os.ReadFile(s.File)
		if errors.Is(ferr, fs.ErrNotExist) {
			return c, ErrNone
		}
		if ferr != nil {
			return c, ferr
		}
		raw = string(b)
	}
	if err := json.Unmarshal([]byte(raw), &c); err != nil || c.Cookie == "" {
		return Creds{}, ErrNone
	}
	return c, nil
}

// Delete removes the credentials from both places.
func (s Store) Delete() error {
	kerr, ferr := keyring.Delete(service, s.account()), os.Remove(s.File)
	if errors.Is(kerr, keyring.ErrNotFound) {
		kerr = nil
	}
	if errors.Is(ferr, fs.ErrNotExist) {
		ferr = nil
	}
	return errors.Join(kerr, ferr)
}
