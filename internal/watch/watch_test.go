package watch

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func collect(ch <-chan string, window time.Duration) []string {
	var got []string
	deadline := time.After(window)
	for {
		select {
		case p, ok := <-ch:
			if !ok {
				return got
			}
			got = append(got, filepath.Base(p))
		case <-deadline:
			return got
		}
	}
}

func TestBurstEmitsOnce(t *testing.T) {
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch, err := Dir(ctx, dir, 150*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	f := filepath.Join(dir, "main.cpp")
	// an editor save: several writes plus a chmod in quick succession
	for i := 0; i < 5; i++ {
		os.WriteFile(f, []byte{byte('a' + i)}, 0o644)
		os.Chmod(f, 0o600)
		time.Sleep(10 * time.Millisecond)
	}
	if got := collect(ch, 800*time.Millisecond); len(got) != 1 || got[0] != "main.cpp" {
		t.Fatalf("a burst must produce exactly one event, got %v", got)
	}
	// a later save is a new event
	os.WriteFile(f, []byte("again"), 0o644)
	if got := collect(ch, 800*time.Millisecond); len(got) != 1 {
		t.Fatalf("second save: %v", got)
	}
}

func TestSaveByRenameAndOtherFiles(t *testing.T) {
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch, _ := Dir(ctx, dir, 100*time.Millisecond)
	f := filepath.Join(dir, "main.py")
	os.WriteFile(f, []byte("v1"), 0o644)
	collect(ch, 400*time.Millisecond) // drain the creation event
	// write-to-temp then rename over the target (how some editors save)
	os.WriteFile(f+".tmp", []byte("v2"), 0o644)
	os.Rename(f+".tmp", f)
	got := collect(ch, 600*time.Millisecond)
	found := false
	for _, g := range got {
		if g == "main.py" {
			found = true
		}
	}
	if !found {
		t.Fatalf("rename-over save must report main.py, got %v", got)
	}
}

func TestStopsWhenCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	ch, err := Dir(ctx, t.TempDir(), 50*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("no events expected")
		}
	case <-time.After(time.Second):
		t.Fatal("channel should close after cancel")
	}
	if _, err := Dir(context.Background(), "/no/such/dir", time.Millisecond); err == nil {
		t.Fatal("missing dir must error")
	}
}
