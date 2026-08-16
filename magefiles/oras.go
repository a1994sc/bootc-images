package main

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

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

func GenerateDiffID(dir, file string) (dig digest.Digest, filePath string, err error) {
	info, err := os.Stat(file)
	if err != nil {
		return ocispec.DescriptorEmptyJSON.Digest, "", err
	}
	tmpDir, err := MakeTempDir("/tmp")
	if err != nil {
		return ocispec.DescriptorEmptyJSON.Digest, "", err
	}

	temp := filepath.Join(tmpDir, fmt.Sprintf("%s.tar", filepath.Base(file)))
	out, err := os.Create(temp)
	if err != nil {
		return ocispec.DescriptorEmptyJSON.Digest, "", err
	}
	defer out.Close()

	tw := tar.NewWriter(out)
	defer tw.Close()

	hdr, err := tar.FileInfoHeader(info, "")
	if err != nil {
		return ocispec.DescriptorEmptyJSON.Digest, temp, err
	}
	hdr.Name = strings.TrimPrefix(file, fmt.Sprintf("%s/", dir))
	if err := tw.WriteHeader(hdr); err != nil {
		return ocispec.DescriptorEmptyJSON.Digest, temp, err
	}

	src, err := os.Open(file)
	if err != nil {
		return ocispec.DescriptorEmptyJSON.Digest, temp, err
	}
	defer src.Close()

	if _, err := io.Copy(tw, src); err != nil {
		return ocispec.DescriptorEmptyJSON.Digest, temp, err
	}

	data, err := os.ReadFile(file)
	if err != nil {
		return ocispec.DescriptorEmptyJSON.Digest, temp, err
	}

	return digest.FromBytes(data), temp, nil
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

		id, f, err := GenerateDiffID(folder, path)
		if err != nil {
			return err
		}
		defer func() {
			os.RemoveAll(filepath.Dir(f))
		}()
		if id == ocispec.DescriptorEmptyJSON.Digest {
			return nil
		}

		tgz, err := os.Open(f)
		if err != nil {
			return err
		}

		info, err := tgz.Stat()
		if err != nil {
			return err
		}

		dgst, err := digest.FromReader(tgz) // consumes f — need to reopen/seek before the real push
		if err != nil {
			return err
		}
		if _, err := tgz.Seek(0, io.SeekStart); err != nil {
			return err
		}

		config.RootFS.DiffIDs = append(config.RootFS.DiffIDs, dgst)

		fileName, err := filepath.Rel(folder, path)
		if err != nil {
			return err
		}

		file := ocispec.Descriptor{
			MediaType: ocispec.MediaTypeImageLayer,
			Digest:    dgst,
			Size:      info.Size(),
			Annotations: map[string]string{
				ocispec.AnnotationTitle:   fileName,
				ocispec.AnnotationCreated: format,
			},
		}

		if err := store.Push(ctx, file, tgz); err != nil {
			return err
		}

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
