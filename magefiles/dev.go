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
	"path/filepath"

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

func (Dev) Digest(ctx context.Context) (err error) {
	tmpDir, err := MakeTempDir("/tmp")
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, os.RemoveAll(tmpDir))
	}()

	dir, err := os.Getwd()
	if err != nil {
		return err
	}

	local, err := UploadDirectory(ctx, tmpDir, filepath.Join(dir, ".direnv", "rpm"), "latest")
	if err != nil {
		return err
	}
	remote, err := RemoteRepo("localhost:5000/rpm/package")
	if err != nil {
		return err
	}

	desc, err := oras.Copy(ctx, local, "latest", remote, "latest", oras.DefaultCopyOptions)

	fmt.Println(desc.Digest)

	return err
}
