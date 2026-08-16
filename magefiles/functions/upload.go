package functions

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/a1994sc/bootc-images/magefiles/files"
	"github.com/a1994sc/bootc-images/magefiles/utils"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2"
)

// Process will take in a directory and create a docker compatible image used as an image volume mount
func Process(ctx context.Context, path string, repo *string, version *string, auth *string) (err error) {
	registry, tag, authFile := "localhost:5000/rpm", "latest", "~/.docker/config.json"
	if repo != nil {
		registry = *repo
	}
	if version != nil {
		tag = *version
	}
	if auth != nil {
		authFile = *auth
	}

	tmpDir, err := utils.MakeTempDir("/tmp")
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, os.RemoveAll(tmpDir))
	}()

	authFile, err = utils.GetAbsHomePath(authFile)
	if err != nil {
		return err
	}

	local, err := files.UploadDirectory(ctx, tmpDir, path, tag)
	if err != nil {
		return err
	}
	remote, err := files.RemoteRepo(authFile, registry)
	if err != nil {
		return err
	}

	copyOpts := oras.DefaultCopyOptions
	copyOpts.PreCopy = func(_ context.Context, desc ocispec.Descriptor) error {
		fmt.Printf("pushing %s (%s, %d bytes)\n", desc.Digest, desc.MediaType, desc.Size)
		return nil
	}
	copyOpts.PostCopy = func(_ context.Context, desc ocispec.Descriptor) error {
		fmt.Printf("pushed %s\n", desc.Digest)
		return nil
	}
	copyOpts.OnCopySkipped = func(_ context.Context, desc ocispec.Descriptor) error {
		fmt.Printf("skipping %s, already present on the registry\n", desc.Digest)
		return nil
	}

	desc, err := oras.Copy(ctx, local, tag, remote, tag, copyOpts)

	fmt.Println(desc.Digest)

	return err
}
