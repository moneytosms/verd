package main

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// uninstallPaths is everything verd owns on disk. The Workspace (your Solutions) is never touched.
type uninstallPaths struct {
	exe, config, data, cache, workspace string
}

// uninstallCmd removes verd. Modes: --all (binary, config, data, cache), --backup DIR (archive config and
// data into DIR first, then --all), --keep (binary only). With no mode it asks. --yes skips the confirmation.
func uninstallCmd(out io.Writer, in io.Reader, args []string, p uninstallPaths) error {
	var mode, backupDir string
	yes := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--all", "--keep":
			mode = args[i][2:]
		case "--backup":
			if i+1 >= len(args) {
				return &exitError{2, "usage: verd uninstall [--all | --keep | --backup DIR] [--yes]"}
			}
			mode, backupDir = "backup", args[i+1]
			i++
		case "--yes", "-y":
			yes = true
		default:
			return &exitError{2, "usage: verd uninstall [--all | --keep | --backup DIR] [--yes]"}
		}
	}
	for _, d := range []string{p.config, p.data, p.cache} {
		if filepath.Base(d) != "verd" {
			return &exitError{2, fmt.Sprintf("refusing to remove %q: not a verd directory", d)}
		}
	}
	r := bufio.NewReader(in)
	if mode == "" {
		fmt.Fprintf(out, "How should verd be removed?\n  1) everything: binary, config, data, cache\n  2) back up config and data first, then remove everything\n  3) binary only: keep config and data\nChoice [1/2/3]: ")
		line, _ := r.ReadString('\n')
		switch strings.TrimSpace(line) {
		case "1":
			mode = "all"
		case "2":
			mode = "backup"
			def, _ := os.UserHomeDir()
			fmt.Fprintf(out, "Backup directory [%s]: ", def)
			line, _ = r.ReadString('\n')
			if backupDir = strings.TrimSpace(line); backupDir == "" {
				backupDir = def
			}
		case "3":
			mode = "keep"
		default:
			return &exitError{2, "no choice made; nothing removed (or pass --all, --keep, --backup DIR)"}
		}
	}
	remove := []string{p.exe}
	if mode != "keep" {
		remove = append(remove, p.config, p.data, p.cache)
	}
	fmt.Fprintln(out, "will remove:")
	for _, d := range remove {
		fmt.Fprintln(out, "  "+d)
	}
	if mode == "backup" {
		fmt.Fprintf(out, "backup of %s and %s goes to %s first\n", p.config, p.data, backupDir)
	}
	if p.workspace != "" {
		fmt.Fprintln(out, "not touched: your Solutions in", p.workspace)
	}
	if !yes {
		fmt.Fprint(out, "Continue? [y/N]: ")
		if line, _ := r.ReadString('\n'); !slices.Contains([]string{"y", "yes"}, strings.ToLower(strings.TrimSpace(line))) {
			fmt.Fprintln(out, "cancelled; nothing removed")
			return &exitError{1, ""}
		}
	}
	if mode == "backup" {
		file, err := backup(backupDir, p, time.Now())
		if err != nil {
			return &exitError{2, "backup failed, nothing removed: " + err.Error()}
		}
		fmt.Fprintln(out, "backup:", file)
	}
	for _, d := range remove[1:] {
		if err := os.RemoveAll(d); err != nil {
			return err
		}
	}
	if err := os.Remove(p.exe); err != nil {
		return &exitError{1, fmt.Sprintf("removed data, but could not delete %s (%v): delete it by hand", p.exe, err)}
	}
	fmt.Fprintln(out, "verd removed")
	return nil
}

// backup writes verd-backup-<time>.tar.gz into dir with config/ and data/ trees, and returns its path.
func backup(dir string, p uninstallPaths, now time.Time) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, "verd-backup-"+now.Format("20060102-150405")+".tar.gz")
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return "", err
	}
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	for _, t := range []struct{ name, src string }{{"config", p.config}, {"data", p.data}} {
		err = filepath.WalkDir(t.src, func(path string, d fs.DirEntry, werr error) error {
			if werr != nil {
				return werr
			}
			if !d.IsDir() && !d.Type().IsRegular() {
				return nil // sockets, symlinks
			}
			info, _ := d.Info()
			h, _ := tar.FileInfoHeader(info, "")
			rel, _ := filepath.Rel(t.src, path)
			h.Name = filepath.ToSlash(filepath.Join(t.name, rel))
			if werr = tw.WriteHeader(h); werr != nil || d.IsDir() {
				return werr
			}
			src, werr := os.Open(path)
			if werr != nil {
				return werr
			}
			defer src.Close()
			_, werr = io.Copy(tw, src)
			return werr
		})
		if err != nil && !os.IsNotExist(err) {
			break
		}
		err = nil
	}
	for _, c := range []io.Closer{tw, gz, f} {
		if cerr := c.Close(); err == nil {
			err = cerr
		}
	}
	if err != nil {
		os.Remove(path)
		return "", err
	}
	return path, nil
}
