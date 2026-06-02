// Package vfs implements an in-memory virtual filesystem for compilation sessions.
// Source files are kept in memory to avoid disk I/O; only explicit persist calls write to disk.
package vfs

import (
	"fmt"
	"sync"
	"time"
)

// File represents a single file in the virtual filesystem.
type File struct {
	Name    string
	Data    []byte
	ModTime time.Time
}

// Workspace is a session-scoped in-memory filesystem.
type Workspace struct {
	mu    sync.RWMutex
	files map[string]*File
}

// New creates an empty Workspace.
func New() *Workspace {
	return &Workspace{files: make(map[string]*File)}
}

// Write creates or overwrites a file.
func (w *Workspace) Write(name string, data []byte) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.files[name] = &File{
		Name:    name,
		Data:    append([]byte(nil), data...),
		ModTime: time.Now(),
	}
}

// Read returns the contents of a file, or an error if it does not exist.
func (w *Workspace) Read(name string) ([]byte, error) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	f, ok := w.files[name]
	if !ok {
		return nil, fmt.Errorf("vfs: file not found: %s", name)
	}
	out := make([]byte, len(f.Data))
	copy(out, f.Data)
	return out, nil
}

// Delete removes a file. No-op if the file does not exist.
func (w *Workspace) Delete(name string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	delete(w.files, name)
}

// List returns all filenames in the workspace.
func (w *Workspace) List() []string {
	w.mu.RLock()
	defer w.mu.RUnlock()
	names := make([]string, 0, len(w.files))
	for n := range w.files {
		names = append(names, n)
	}
	return names
}

// Exists checks whether a file exists.
func (w *Workspace) Exists(name string) bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	_, ok := w.files[name]
	return ok
}
