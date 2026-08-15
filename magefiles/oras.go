package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gabriel-vasile/mimetype"
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

func RemoteRepo(reference string) (oras.Target, error) {
	store, err := credentials.NewStoreFromDocker(credentials.StoreOptions{
		DetectDefaultNativeStore: true,
	})
	if err != nil {
		return nil, err
	}
	repo, err := remote.NewRepository(reference)
	if err != nil {
		return nil, err
	}
	repo.PlainHTTP = true // skip TLS for local/insecure registries
	repo.Client = &auth.Client{
		Credential: auth.CredentialFunc(func(ctx context.Context, host string) (auth.Credential, error) {
			return store.Get(ctx, host)
		}),
	}

	return repo, nil
}

func GenerateDiffID(dir, file string) (digest.Digest, error) {
	info, err := os.Stat(file)
	if err != nil {
		return ocispec.DescriptorEmptyJSON.Digest, err
	}
	tmpDir, err := MakeTempDir("/tmp")
	if err != nil {
		return ocispec.DescriptorEmptyJSON.Digest, err
	}
	defer func() {
		os.RemoveAll(tmpDir)
	}()

	temp := filepath.Join(tmpDir, "temp.tar")
	out, err := os.Create(temp)
	if err != nil {
		return ocispec.DescriptorEmptyJSON.Digest, err
	}
	defer out.Close()

	tw := tar.NewWriter(out)
	defer tw.Close()

	hdr, err := tar.FileInfoHeader(info, "")
	if err != nil {
		return ocispec.DescriptorEmptyJSON.Digest, err
	}
	hdr.Name = strings.TrimPrefix(file, fmt.Sprintf("%s/", dir))
	if err := tw.WriteHeader(hdr); err != nil {
		return ocispec.DescriptorEmptyJSON.Digest, err
	}

	src, err := os.Open(file)
	if err != nil {
		return ocispec.DescriptorEmptyJSON.Digest, err
	}
	defer src.Close()

	if _, err := io.Copy(tw, src); err != nil {
		return ocispec.DescriptorEmptyJSON.Digest, err
	}

	data, err := os.ReadFile(file)
	if err != nil {
		return ocispec.DescriptorEmptyJSON.Digest, err
	}

	return digest.FromBytes(data), nil
}

func GenerateGZipTarBall(dir string, file string) (string, string, error) {
	tmpDir, err := MakeTempDir("/tmp")
	if err != nil {
		return "", "", err
	}
	info, err := os.Stat(file)
	if err != nil {
		return "", "", err
	}

	filePath := filepath.Join(tmpDir, fmt.Sprintf("%s.tar.gz", filepath.Base(file)))

	out, err := os.Create(filePath)
	if err != nil {
		return "", "", err
	}
	defer out.Close()

	gw := gzip.NewWriter(out)
	tw := tar.NewWriter(gw)

	hdr, err := tar.FileInfoHeader(info, "")
	if err != nil {
		return "", "", err
	}
	hdr.Name = strings.TrimPrefix(file, fmt.Sprintf("%s/", dir))

	if err := tw.WriteHeader(hdr); err != nil {
		return "", "", err
	}

	src, err := os.Open(file)
	if err != nil {
		return "", "", err
	}
	defer src.Close()

	if _, err := io.Copy(tw, src); err != nil {
		return "", "", err
	}

	// Close explicitly, in order: tar footer must be written before gzip is finalized.
	if err := tw.Close(); err != nil {
		return "", "", err
	}
	if err := gw.Close(); err != nil {
		return "", "", err
	}

	return filePath, tmpDir, nil
}

func UploadDirectory(ctx context.Context, ociDir, folder, tag string) (*file.Store, error) {
	store, err := file.New(ociDir)
	if err != nil {
		return nil, err
	}

	files := []ocispec.Descriptor{}
	config := ocispec.Image{
		Platform: ocispec.Platform{
			OS:           OSLinux,
			Architecture: OSArchAMD64,
		},
		Created: &static,
		RootFS: ocispec.RootFS{
			Type:    "layers",
			DiffIDs: []digest.Digest{},
		},
	}

	err = filepath.WalkDir(folder, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		filePath, tmpDir, err := GenerateGZipTarBall(folder, path)
		if err != nil {
			return err
		}
		defer func() {
			os.RemoveAll(tmpDir)
		}()

		mtype, err := mimetype.DetectFile(filePath)
		if err != nil {
			return err
		}

		mt, _, _ := strings.Cut(mtype.String(), ";")

		file, err := store.Add(ctx, filepath.Base(filePath), mt, filePath)
		if err != nil {
			return err
		}

		fileName, err := filepath.Rel(folder, path)
		if err != nil {
			return err
		}

		file.Annotations = map[string]string{
			ocispec.AnnotationTitle:   fileName,
			ocispec.AnnotationCreated: format,
		}

		id, err := GenerateDiffID(folder, path)
		if err != nil {
			return err
		}

		if id == ocispec.DescriptorEmptyJSON.Digest {
			return nil
		}

		config.RootFS.DiffIDs = append(config.RootFS.DiffIDs, id)

		files = append(files, file)

		return nil
	})

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
