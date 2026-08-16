// Copyright 2026 colonel-byte
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build mage
// +build mage

package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/a1994sc/bootc-images/magefiles/files"
	"github.com/a1994sc/bootc-images/magefiles/utils"
	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
	"oras.land/oras-go/v2"
)

type (
	Dev mg.Namespace
)

// Tidy just runs the module tidy
func (Dev) Tidy() error {
	fmt.Println("Running tidy")
	return sh.RunV(
		"go",
		"mod",
		"tidy",
	)
}

// Process will take in a directory and create a docker compatible image used as an image volume mount
func Process(ctx context.Context, path string, repo *string, version *string) (err error) {
	registry, tag := "localhost:5000/rpm", "latest"
	if repo != nil {
		registry = *repo
	}
	if version != nil {
		tag = *version
	}

	tmpDir, err := utils.MakeTempDir("/tmp")
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, os.RemoveAll(tmpDir))
	}()

	local, err := files.UploadDirectory(ctx, tmpDir, path, tag)
	if err != nil {
		return err
	}
	remote, err := files.RemoteRepo(registry)
	if err != nil {
		return err
	}

	desc, err := oras.Copy(ctx, local, tag, remote, tag, oras.DefaultCopyOptions)

	fmt.Println(desc.Digest)

	return err
}
