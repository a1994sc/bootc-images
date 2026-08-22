package utils

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMakeTempDir(t *testing.T) {
	t.Run("empty base path uses the system default", func(t *testing.T) {
		dir, err := MakeTempDir("")
		if err != nil {
			t.Fatalf("MakeTempDir() error = %v", err)
		}
		defer os.RemoveAll(dir)

		info, err := os.Stat(dir)
		if err != nil {
			t.Fatalf("expected temp dir to exist, stat error = %v", err)
		}
		if !info.IsDir() {
			t.Fatalf("expected %s to be a directory", dir)
		}
		if !strings.HasPrefix(filepath.Base(dir), tmpPathPrefix) {
			t.Fatalf("expected dir name %q to have prefix %q", filepath.Base(dir), tmpPathPrefix)
		}
	})

	t.Run("base path is created when missing", func(t *testing.T) {
		base := filepath.Join(t.TempDir(), "does", "not", "exist", "yet")

		dir, err := MakeTempDir(base)
		if err != nil {
			t.Fatalf("MakeTempDir() error = %v", err)
		}
		defer os.RemoveAll(dir)

		if !strings.HasPrefix(dir, base) {
			t.Fatalf("expected %q to be created under %q", dir, base)
		}
		if _, err := os.Stat(dir); err != nil {
			t.Fatalf("expected temp dir to exist, stat error = %v", err)
		}
	})

	t.Run("base path already existing is reused", func(t *testing.T) {
		base := t.TempDir()

		dir, err := MakeTempDir(base)
		if err != nil {
			t.Fatalf("MakeTempDir() error = %v", err)
		}
		defer os.RemoveAll(dir)

		if filepath.Dir(dir) != base {
			t.Fatalf("expected temp dir %q to be created directly under %q", dir, base)
		}
	})

	t.Run("repeated calls return distinct directories", func(t *testing.T) {
		base := t.TempDir()

		first, err := MakeTempDir(base)
		if err != nil {
			t.Fatalf("MakeTempDir() error = %v", err)
		}
		defer os.RemoveAll(first)

		second, err := MakeTempDir(base)
		if err != nil {
			t.Fatalf("MakeTempDir() error = %v", err)
		}
		defer os.RemoveAll(second)

		if first == second {
			t.Fatalf("expected distinct directories, got %q twice", first)
		}
	})
}

func TestCreateDirectory(t *testing.T) {
	t.Run("creates a missing directory", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "sub", "dir")

		if err := CreateDirectory(path, ReadWriteExecuteUser); err != nil {
			t.Fatalf("CreateDirectory() error = %v", err)
		}

		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("expected directory to exist, stat error = %v", err)
		}
		if !info.IsDir() {
			t.Fatalf("expected %s to be a directory", path)
		}
	})

	t.Run("is a no-op for an existing directory", func(t *testing.T) {
		path := t.TempDir()

		if err := CreateDirectory(path, ReadWriteExecuteUser); err != nil {
			t.Fatalf("CreateDirectory() error = %v", err)
		}

		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("expected directory to still exist, stat error = %v", err)
		}
		if !info.IsDir() {
			t.Fatalf("expected %s to still be a directory", path)
		}
	})
}

func TestInvalidPath(t *testing.T) {
	t.Run("returns true for a path that does not exist", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "nope")

		if !InvalidPath(path) {
			t.Fatalf("expected InvalidPath(%q) = true", path)
		}
	})

	t.Run("returns false for a path that exists", func(t *testing.T) {
		path := t.TempDir()

		if InvalidPath(path) {
			t.Fatalf("expected InvalidPath(%q) = false", path)
		}
	})

	t.Run("returns false when stat fails with a permission error", func(t *testing.T) {
		if os.Getuid() == 0 {
			t.Skip("permission checks are bypassed when running as root")
		}

		parent := t.TempDir()
		child := filepath.Join(parent, "child")
		if err := os.Mkdir(child, 0700); err != nil {
			t.Fatalf("failed to set up child dir: %v", err)
		}

		// Remove execute permission on the parent so stat on the child fails
		// with a permission error rather than "not exist".
		if err := os.Chmod(parent, 0600); err != nil {
			t.Fatalf("failed to chmod parent dir: %v", err)
		}
		defer os.Chmod(parent, 0700) // restore so t.TempDir() cleanup can remove it

		if InvalidPath(child) {
			t.Fatalf("expected InvalidPath(%q) = false for a permission error", child)
		}
	})
}
