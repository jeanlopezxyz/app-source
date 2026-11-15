package database

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadPasswordTrimsAndRereads(t *testing.T) {
	path := filepath.Join(t.TempDir(), "password")
	if err := os.WriteFile(path, []byte("first\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := ReadPassword(path)
	if err != nil || got != "first" {
		t.Fatalf("ReadPassword = %q, %v; want first", got, err)
	}

	// Simulates the kubelet updating the mounted Secret after a rotation.
	if err := os.WriteFile(path, []byte("second"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err = ReadPassword(path)
	if err != nil || got != "second" {
		t.Fatalf("after rotation ReadPassword = %q, %v; want second", got, err)
	}
}

func TestReadPasswordErrors(t *testing.T) {
	dir := t.TempDir()
	if _, err := ReadPassword(filepath.Join(dir, "missing")); err == nil {
		t.Error("expected an error for a missing file")
	}
	empty := filepath.Join(dir, "empty")
	if err := os.WriteFile(empty, []byte(" \n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadPassword(empty); err == nil {
		t.Error("expected an error for an empty file")
	}
}
