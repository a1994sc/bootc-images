package files

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/a1994sc/bootc-images/magefiles/utils"
	digest "github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/content/file"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/credentials"
)

const (
	OSLinux = "linux"

	OSArchAMD64 = "amd64"
	OSArchARM64 = "arm64"
)

var (
	static = time.Date(2000, time.January, 1, 0, 0, 0, 0, time.UTC)
	format = time.Date(2000, time.January, 1, 0, 0, 0, 0, time.UTC).Format(time.RFC3339)
)

func RemoteRepo(authFile, reference string) (oras.Target, error) {
	store, err := credentials.NewStore(authFile, credentials.StoreOptions{
		DetectDefaultNativeStore: true,
	})
	if err != nil {
		return nil, err
	}
	repo, err := remote.NewRepository(reference)
	if err != nil {
		return nil, err
	}
	repo.PlainHTTP = isLocalRegistry(repo.Reference.Host()) // skip TLS for local/insecure registries
	repo.Client = &auth.Client{
		Credential: credentials.Credential(store),
	}

	return repo, nil
}

// isLocalRegistry reports whether host is a local, insecure registry (e.g.
// "localhost:5000") that should be accessed over plain HTTP instead of TLS.
func isLocalRegistry(host string) bool {
	hostname := host
	if h, _, err := net.SplitHostPort(host); err == nil {
		hostname = h
	}
	return hostname == "localhost" || hostname == "127.0.0.1" || hostname == "::1"
}

// GenerateDiffID tars file into tmpDir and returns the digest and size of the
// resulting tar stream, computed in a single pass while it is written to disk.
func GenerateDiffID(tmpDir, dir, file string) (dig digest.Digest, filePath string, size int64, err error) {
	info, err := os.Stat(file)
	if err != nil {
		return "", "", 0, err
	}

	rel := strings.TrimPrefix(file, dir+"/")
	temp := filepath.Join(tmpDir, strings.ReplaceAll(rel, string(filepath.Separator), "_")+".tar")
	out, err := os.Create(temp)
	if err != nil {
		return "", "", 0, err
	}
	defer out.Close()

	digester := digest.Canonical.Digester()
	tw := tar.NewWriter(io.MultiWriter(out, digester.Hash()))

	hdr, err := tar.FileInfoHeader(info, "")
	if err != nil {
		return "", temp, 0, err
	}
	hdr.Name = rel
	if err := tw.WriteHeader(hdr); err != nil {
		return "", temp, 0, err
	}

	src, err := os.Open(file)
	if err != nil {
		return "", temp, 0, err
	}
	defer src.Close()

	if _, err := io.Copy(tw, src); err != nil {
		return "", temp, 0, err
	}
	if err := tw.Close(); err != nil {
		return "", temp, 0, err
	}

	fi, err := out.Stat()
	if err != nil {
		return "", temp, 0, err
	}

	return digester.Digest(), temp, fi.Size(), nil
}

func UploadDirectory(ctx context.Context, ociDir, folder, tag string) (_ *file.Store, err error) {
	store, err := file.New(ociDir)
	if err != nil {
		return nil, err
	}

	tmpDir, err := utils.MakeTempDir("/tmp")
	if err != nil {
		return nil, err
	}
	defer func() {
		err = errors.Join(err, os.RemoveAll(tmpDir))
	}()

	files := []ocispec.Descriptor{}
	config := ocispec.Image{
		Platform: ocispec.Platform{
			OS:           OSLinux,
			Architecture: runtime.GOARCH,
		},
		Created: &static,
		History: []ocispec.History{},
		RootFS: ocispec.RootFS{
			Type:    "layers",
			DiffIDs: []digest.Digest{},
		},
	}

	fmt.Printf("walking %s\n", folder)

	err = filepath.WalkDir(folder, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		dgst, f, size, err := GenerateDiffID(tmpDir, folder, path)
		if err != nil {
			return err
		}

		tgz, err := os.Open(f)
		if err != nil {
			return err
		}
		defer tgz.Close()

		fileName, err := filepath.Rel(folder, path)
		if err != nil {
			return err
		}

		config.History = append(config.History, ocispec.History{
			Created:   &static,
			Comment:   "xyz.adrp.file.v0",
			CreatedBy: fmt.Sprintf("ADD %s /", fileName),
		})
		config.RootFS.DiffIDs = append(config.RootFS.DiffIDs, dgst)

		layer := ocispec.Descriptor{
			MediaType: ocispec.MediaTypeImageLayer,
			Digest:    dgst,
			Size:      size,
			Annotations: map[string]string{
				ocispec.AnnotationTitle:   fileName,
				ocispec.AnnotationCreated: format,
			},
		}

		fmt.Printf("pushing %s (%s, %d bytes)\n", fileName, dgst, size)
		if err := store.Push(ctx, layer, tgz); err != nil {
			return err
		}

		files = append(files, layer)

		return nil
	})
	if err != nil {
		return nil, err
	}

	fmt.Printf("pushed %d files from %s\n", len(files), folder)

	configBytes, err := json.Marshal(config)
	if err != nil {
		return nil, err
	}
	configDesc := content.NewDescriptorFromBytes(ocispec.MediaTypeImageConfig, configBytes)
	err = store.Push(ctx, configDesc, bytes.NewBuffer(configBytes))
	if err != nil {
		return nil, err
	}

	manifestDesc, err := oras.PackManifest(
		ctx,
		store,
		oras.PackManifestVersion1_1,
		"application/vnd.oci.image.manifest.v1+json",
		oras.PackManifestOptions{
			Layers:           files,
			ConfigDescriptor: &configDesc,
			ManifestAnnotations: map[string]string{
				ocispec.AnnotationCreated: format,
			},
		},
	)
	if err != nil {
		return nil, err
	}

	manifestDesc.Platform = &ocispec.Platform{
		Architecture: OSArchAMD64,
		OS:           OSLinux,
	}

	if err := store.Tag(ctx, manifestDesc, tag); err != nil {
		return nil, err
	}

	return store, nil
}
