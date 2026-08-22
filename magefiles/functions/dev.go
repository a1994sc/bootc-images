package functions

import (
	"fmt"

	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

type (
	Dev mg.Namespace
)

// Tidy just runs the module tidy
func Tidy() error {
	fmt.Println("Running tidy")
	return sh.RunV(
		"go",
		"mod",
		"tidy",
	)
}

// Vendor just runs the module vendor
func Vendor() error {
	return sh.RunV(
		"go",
		"mod",
		"vendor",
	)
}
