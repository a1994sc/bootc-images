package functions

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/a1994sc/bootc-images/magefiles/utils"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/zoci/archive"
	"github.com/zarf-dev/zarf/src/pkg/zoci/image"
)

// Process will take in a directory and create a docker compatible image used as an image volume mount
func Process(ctx context.Context, path string, repo *string, version *string, platOS *string, platArch *string, tarBall *string) (err error) {
	registry, tag, osRef, arch, tar := "localhost:5000/rpm", "latest", "linux", "amd64", ""
	if repo != nil {
		registry = *repo
	}
	if version != nil {
		tag = *version
	}
	imageReg := fmt.Sprintf("%s:%s", registry, tag)
	if platOS != nil {
		osRef = *platOS
	}
	if platArch != nil {
		arch = *platArch
	}
	if tarBall != nil {
		tar = *tarBall
	} else if tar == "" {
		tar = archive.ImageRefToTar(imageReg)
	}

	tmpDir, err := utils.MakeTempDir("/tmp")
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, os.RemoveAll(tmpDir))
	}()

	iv, err := image.New(tmpDir, osRef, arch)
	if err != nil {
		return err
	}
	defer func() {
		if err := iv.Clean(); err != nil {
			logger.From(ctx).Debug("failed to clean image volume workspace", "error", err)
		}
		if err := os.RemoveAll(tmpDir); err != nil {
			logger.From(ctx).Debug("failed to remove staging directory", "error", err)
		}
	}()

	if err := iv.AddDirectory(ctx, path, imageReg); err != nil {
		return err
	}

	out, err := os.Create(tar)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := out.Close(); closeErr != nil {
			logger.From(ctx).Debug("failed to close image volume archive", "error", closeErr)
		}
	}()

	if err := iv.WriteTar(ctx, imageReg, out); err != nil {
		return err
	}

	return err
}
