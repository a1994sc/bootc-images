package utils

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetAbsHomePath(t *testing.T) {
	t.Run("path without a tilde prefix is returned unchanged", func(t *testing.T) {
		for _, path := range []string{"/foo/bar", "relative/path", "", "foo~bar"} {
			got, err := GetAbsHomePath(path)
			if err != nil {
				t.Fatalf("GetAbsHomePath(%q) error = %v", path, err)
			}
			if got != path {
				t.Fatalf("GetAbsHomePath(%q) = %q, want %q", path, got, path)
			}
		}
	})

	t.Run("leading tilde is replaced with the user's home directory", func(t *testing.T) {
		home, err := os.UserHomeDir()
		if err != nil {
			t.Fatalf("failed to determine home dir for test setup: %v", err)
		}

		tests := []struct {
			path string
			want string
		}{
			{path: "~", want: home},
			{path: "~/foo", want: filepath.Join(home, "foo")},
			{path: "~/foo/bar", want: filepath.Join(home, "foo", "bar")},
		}

		for _, tt := range tests {
			got, err := GetAbsHomePath(tt.path)
			if err != nil {
				t.Fatalf("GetAbsHomePath(%q) error = %v", tt.path, err)
			}
			if got != tt.want {
				t.Fatalf("GetAbsHomePath(%q) = %q, want %q", tt.path, got, tt.want)
			}
		}
	})

	t.Run("only the leading tilde is substituted, not a literal path join", func(t *testing.T) {
		home, err := os.UserHomeDir()
		if err != nil {
			t.Fatalf("failed to determine home dir for test setup: %v", err)
		}

		// "~foo" is not "~/foo": there is no separator after the tilde, so the
		// replacement is a straight string substitution, not a path join. This
		// documents current behavior rather than shell-style user expansion
		// (e.g. "~otheruser" resolving to that user's home directory).
		got, err := GetAbsHomePath("~foo")
		if err != nil {
			t.Fatalf("GetAbsHomePath() error = %v", err)
		}
		want := home + "foo"
		if got != want {
			t.Fatalf("GetAbsHomePath(%q) = %q, want %q", "~foo", got, want)
		}
	})

	t.Run("returns an error when the home directory cannot be determined", func(t *testing.T) {
		t.Setenv("HOME", "")

		_, err := GetAbsHomePath("~/foo")
		if err == nil {
			t.Fatal("expected an error when HOME is unset, got nil")
		}
	})
}
