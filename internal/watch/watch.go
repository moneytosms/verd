// Package watch reports saved files in a directory, debounced.
package watch

import (
	"context"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Dir watches dir (not individual files, so editors that save by rename keep working) and sends
// the absolute path of each file written or created, once per burst: a path is emitted after no
// event for it has arrived for debounce. The channel closes when ctx is done.
func Dir(ctx context.Context, dir string, debounce time.Duration) (<-chan string, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	if err := w.Add(dir); err != nil {
		w.Close()
		return nil, err
	}
	out := make(chan string, 8)
	go func() {
		defer close(out)
		defer w.Close()
		var mu sync.Mutex
		timers := map[string]*time.Timer{}
		defer func() {
			mu.Lock()
			for _, t := range timers {
				t.Stop()
			}
			mu.Unlock()
		}()
		for {
			select {
			case <-ctx.Done():
				return
			case ev, ok := <-w.Events:
				if !ok {
					return
				}
				if !ev.Has(fsnotify.Write) && !ev.Has(fsnotify.Create) {
					continue // chmod, remove, rename-away
				}
				path, _ := filepath.Abs(ev.Name)
				mu.Lock()
				if t, ok := timers[path]; ok {
					t.Reset(debounce)
				} else {
					timers[path] = time.AfterFunc(debounce, func() {
						mu.Lock()
						delete(timers, path)
						mu.Unlock()
						select {
						case out <- path:
						case <-ctx.Done():
						}
					})
				}
				mu.Unlock()
			case <-w.Errors:
			}
		}
	}()
	return out, nil
}
