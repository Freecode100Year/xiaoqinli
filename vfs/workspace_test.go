package vfs

import (
	"testing"
)

func TestWorkspacePathEscape(t *testing.T) {
	w := New()

	// Write a legitimate file
	w.Write("legit.txt", []byte("hello"))

	// Test legitimate read
	data, err := w.Read("legit.txt")
	if err != nil {
		t.Fatalf("legit read failed: %v", err)
	}
	if string(data) != "hello" {
		t.Fatalf("unexpected content: %s", data)
	}

	// Test path traversal attempts
	escapeCases := []string{
		"../etc/passwd",
		"..\\windows\\system32",
		"/absolute/path",
		"C:\\Windows\\System32",
		"subdir/../../etc/passwd",
	}

	for _, path := range escapeCases {
		_, err := w.Read(path)
		if err == nil {
			t.Errorf("expected error for path escape attempt: %s", path)
		}
	}
}