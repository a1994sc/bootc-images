package main

import (
	"context"

	"github.com/a1994sc/bootc-images/magefiles/functions"
)

// Process will take in a directory and create a docker compatible image used as an image volume mount
func Process(ctx context.Context, path string, repo *string, version *string, os *string, arch *string, tar *string, maxLayers *int) (err error) {
	return functions.Process(
		ctx,
		path,
		repo,
		version,
		os,
		arch,
		tar,
		maxLayers,
	)
}
