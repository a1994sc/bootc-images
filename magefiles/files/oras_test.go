package files_test

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/a1994sc/bootc-images/magefiles/files"
	digest "github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/registry/remote"
)

func TestRemoteRepo(t *testing.T) {
	t.Run("local registries use plain HTTP", func(t *testing.T) {
		for _, reference := range []string{
			"localhost:5000/rpm/package",
			"localhost/rpm/package",
			"127.0.0.1:5000/rpm/package",
		} {
			target, err := files.RemoteRepo("", reference)
			if err != nil {
				t.Fatalf("RemoteRepo(%q) error = %v", reference, err)
			}
			repo, ok := target.(*remote.Repository)
			if !ok {
				t.Fatalf("RemoteRepo(%q) returned %T, want *remote.Repository", reference, target)
			}
			if !repo.PlainHTTP {
				t.Fatalf("RemoteRepo(%q).PlainHTTP = false, want true for a local registry", reference)
			}
		}
	})

	t.Run("remote registries use TLS, not plain HTTP", func(t *testing.T) {
		// Regression test: PlainHTTP must not be forced on for real
		// registries. oras-go strips the Authorization header on any
		// cross-origin redirect (see GHSA-vh4v-2xq2-g5cg), and registries
		// like ghcr.io 301-redirect plain HTTP requests to HTTPS. Forcing
		// PlainHTTP here would make every authenticated request silently
		// lose its credentials on that redirect and fail with 401.
		for _, reference := range []string{
			"ghcr.io/owner/repo",
			"registry.example.com:5000/owner/repo",
		} {
			target, err := files.RemoteRepo("", reference)
			if err != nil {
				t.Fatalf("RemoteRepo(%q) error = %v", reference, err)
			}
			repo, ok := target.(*remote.Repository)
			if !ok {
				t.Fatalf("RemoteRepo(%q) returned %T, want *remote.Repository", reference, target)
			}
			if repo.PlainHTTP {
				t.Fatalf("RemoteRepo(%q).PlainHTTP = true, want false for a remote registry", reference)
			}
		}
	})

	t.Run("invalid reference returns an error", func(t *testing.T) {
		_, err := files.RemoteRepo("", "not a valid reference!!")
		if err == nil {
			t.Fatal("expected an error for an invalid reference, got nil")
		}
	})
}

func TestGenerateDiffID(t *testing.T) {
	t.Run("tars the file and returns a digest matching the written bytes", func(t *testing.T) {
		dir := t.TempDir()
		content := []byte("hello world")
		srcPath := filepath.Join(dir, "hello.txt")
		if err := os.WriteFile(srcPath, content, 0600); err != nil {
			t.Fatalf("failed to write source file: %v", err)
		}

		tmpDir := t.TempDir()
		dig, tarPath, size, err := files.GenerateDiffID(tmpDir, dir, srcPath)
		if err != nil {
			t.Fatalf("GenerateDiffID() error = %v", err)
		}

		// The returned digest must match the digest of the tar file actually
		// written to disk, since that is what gets pushed to the store.
		f, err := os.Open(tarPath)
		if err != nil {
			t.Fatalf("failed to open generated tar: %v", err)
		}
		defer f.Close()

		wantDigest, err := digest.FromReader(f)
		if err != nil {
			t.Fatalf("failed to hash generated tar: %v", err)
		}
		if dig != wantDigest {
			t.Fatalf("digest = %s, want %s (digest of the file on disk)", dig, wantDigest)
		}

		info, err := os.Stat(tarPath)
		if err != nil {
			t.Fatalf("failed to stat generated tar: %v", err)
		}
		if size != info.Size() {
			t.Fatalf("size = %d, want %d (actual file size)", size, info.Size())
		}

		// The tar should contain exactly one entry: the source file, named
		// relative to dir, with its original content preserved.
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			t.Fatalf("failed to seek tar: %v", err)
		}
		tr := tar.NewReader(f)
		hdr, err := tr.Next()
		if err != nil {
			t.Fatalf("failed to read tar header: %v", err)
		}
		if hdr.Name != "hello.txt" {
			t.Fatalf("tar entry name = %q, want %q", hdr.Name, "hello.txt")
		}
		gotContent, err := io.ReadAll(tr)
		if err != nil {
			t.Fatalf("failed to read tar entry content: %v", err)
		}
		if !bytes.Equal(gotContent, content) {
			t.Fatalf("tar entry content = %q, want %q", gotContent, content)
		}
		if _, err := tr.Next(); err != io.EOF {
			t.Fatalf("expected exactly one tar entry, found more")
		}
	})

	t.Run("nested files are named relative to dir", func(t *testing.T) {
		dir := t.TempDir()
		nested := filepath.Join(dir, "sub", "nested.txt")
		if err := os.MkdirAll(filepath.Dir(nested), 0700); err != nil {
			t.Fatalf("failed to create nested dir: %v", err)
		}
		if err := os.WriteFile(nested, []byte("nested"), 0600); err != nil {
			t.Fatalf("failed to write nested file: %v", err)
		}

		tmpDir := t.TempDir()
		_, tarPath, _, err := files.GenerateDiffID(tmpDir, dir, nested)
		if err != nil {
			t.Fatalf("GenerateDiffID() error = %v", err)
		}

		f, err := os.Open(tarPath)
		if err != nil {
			t.Fatalf("failed to open generated tar: %v", err)
		}
		defer f.Close()

		tr := tar.NewReader(f)
		hdr, err := tr.Next()
		if err != nil {
			t.Fatalf("failed to read tar header: %v", err)
		}
		if hdr.Name != "sub/nested.txt" {
			t.Fatalf("tar entry name = %q, want %q", hdr.Name, "sub/nested.txt")
		}
	})

	t.Run("returns an error when the source file does not exist", func(t *testing.T) {
		dir := t.TempDir()
		missing := filepath.Join(dir, "missing.txt")

		_, _, _, err := files.GenerateDiffID(t.TempDir(), dir, missing)
		if err == nil {
			t.Fatal("expected an error for a missing source file, got nil")
		}
	})

	t.Run("returns an error when tmpDir does not exist", func(t *testing.T) {
		dir := t.TempDir()
		srcPath := filepath.Join(dir, "hello.txt")
		if err := os.WriteFile(srcPath, []byte("hi"), 0600); err != nil {
			t.Fatalf("failed to write source file: %v", err)
		}

		badTmpDir := filepath.Join(dir, "does-not-exist")
		_, _, _, err := files.GenerateDiffID(badTmpDir, dir, srcPath)
		if err == nil {
			t.Fatal("expected an error for a nonexistent tmpDir, got nil")
		}
	})
}

func TestUploadDirectory(t *testing.T) {
	ctx := context.Background()

	setupFolder := func(t *testing.T) string {
		t.Helper()
		folder := t.TempDir()
		if err := os.WriteFile(filepath.Join(folder, "a.txt"), []byte("file a"), 0600); err != nil {
			t.Fatalf("failed to write a.txt: %v", err)
		}
		sub := filepath.Join(folder, "sub")
		if err := os.MkdirAll(sub, 0700); err != nil {
			t.Fatalf("failed to create sub dir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(sub, "b.txt"), []byte("file b"), 0600); err != nil {
			t.Fatalf("failed to write b.txt: %v", err)
		}
		return folder
	}

	t.Run("builds a tagged OCI image from a directory", func(t *testing.T) {
		folder := setupFolder(t)
		ociDir := t.TempDir()

		store, err := files.UploadDirectory(ctx, ociDir, folder, "latest")
		if err != nil {
			t.Fatalf("UploadDirectory() error = %v", err)
		}
		defer store.Close()

		manifestDesc, err := store.Resolve(ctx, "latest")
		if err != nil {
			t.Fatalf("failed to resolve tag: %v", err)
		}

		rc, err := store.Fetch(ctx, manifestDesc)
		if err != nil {
			t.Fatalf("failed to fetch manifest: %v", err)
		}
		defer rc.Close()

		var manifest ocispec.Manifest
		if err := json.NewDecoder(rc).Decode(&manifest); err != nil {
			t.Fatalf("failed to decode manifest: %v", err)
		}

		if len(manifest.Layers) != 2 {
			t.Fatalf("len(manifest.Layers) = %d, want 2", len(manifest.Layers))
		}

		crc, err := store.Fetch(ctx, manifest.Config)
		if err != nil {
			t.Fatalf("failed to fetch config: %v", err)
		}
		defer crc.Close()

		var config ocispec.Image
		if err := json.NewDecoder(crc).Decode(&config); err != nil {
			t.Fatalf("failed to decode config: %v", err)
		}

		if config.Platform.OS != files.OSLinux {
			t.Fatalf("config.Platform.OS = %q, want %q", config.Platform.OS, files.OSLinux)
		}
		if config.Platform.Architecture != files.OSArchAMD64 {
			t.Fatalf("config.Platform.Architecture = %q, want %q", config.Platform.Architecture, files.OSArchAMD64)
		}
		if len(config.History) != 2 {
			t.Fatalf("len(config.History) = %d, want 2", len(config.History))
		}
		if len(config.RootFS.DiffIDs) != 2 {
			t.Fatalf("len(config.RootFS.DiffIDs) = %d, want 2", len(config.RootFS.DiffIDs))
		}

		// Every layer's diffID must be the digest of its own (uncompressed) tar
		// content, matching what was recorded in the config's RootFS.DiffIDs.
		for i, layer := range manifest.Layers {
			if layer.Digest != config.RootFS.DiffIDs[i] {
				t.Fatalf("layer[%d].Digest = %s, want %s (config.RootFS.DiffIDs[%d])", i, layer.Digest, config.RootFS.DiffIDs[i], i)
			}

			lrc, err := store.Fetch(ctx, layer)
			if err != nil {
				t.Fatalf("failed to fetch layer %d: %v", i, err)
			}
			gotDigest, err := digest.FromReader(lrc)
			lrc.Close()
			if err != nil {
				t.Fatalf("failed to hash layer %d: %v", i, err)
			}
			if gotDigest != layer.Digest {
				t.Fatalf("layer[%d] content digest = %s, want %s", i, gotDigest, layer.Digest)
			}
		}
	})

	t.Run("returns an error when the folder does not exist", func(t *testing.T) {
		ociDir := t.TempDir()
		missing := filepath.Join(t.TempDir(), "does-not-exist")

		_, err := files.UploadDirectory(ctx, ociDir, missing, "latest")
		if err == nil {
			t.Fatal("expected an error for a missing folder, got nil")
		}
	})

	t.Run("returns an error when ociDir cannot be used as a store", func(t *testing.T) {
		folder := setupFolder(t)

		// A regular file cannot be used as the working directory for an OCI
		// file store.
		notADir := filepath.Join(t.TempDir(), "not-a-dir")
		if err := os.WriteFile(notADir, []byte("nope"), 0600); err != nil {
			t.Fatalf("failed to write file: %v", err)
		}

		_, err := files.UploadDirectory(ctx, notADir, folder, "latest")
		if err == nil {
			t.Fatal("expected an error for an invalid ociDir, got nil")
		}
	})
}
