// Copyright 2022 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package linuxtls

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2"
)

func CreateCABundle(dir string) (sets.Set[string], error) {
	bundleFilepath := filepath.Join(dir, "ca-certificates.crt")
	klog.V(4).Infof("Creating CA bundle: %q", bundleFilepath)
	bundleFile, err := os.Create(bundleFilepath)
	if err != nil {
		return nil, fmt.Errorf("failed to create bundle file: %w", err)
	}
	defer bundleFile.Close()

	err = fs.WalkDir(os.DirFS("/"), dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			if path == dir {
				return nil
			}
			return fs.SkipDir
		}

		if filepath.Base(path) == "ca-certificates.crt" {
			return nil
		}

		f, innerErr := os.Open(path)
		if err != nil {
			return fmt.Errorf("failed to read %q: %w", path, innerErr)
		}
		defer f.Close()

		_, innerErr = io.Copy(bundleFile, f)
		if err != nil {
			return fmt.Errorf("failed to copy %q into certificate bundle: %w", path, innerErr)
		}

		_, innerErr = fmt.Fprint(bundleFile, "\n\n")
		if err != nil {
			return fmt.Errorf("failed to copy %q into certificate bundle: %w", path, innerErr)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to walk certificate files to create bundle: %w", err)
	}

	klog.V(4).Infof("Created CA bundle: %q", bundleFilepath)

	return sets.New("ca-certificates.crt"), nil
}
