// Copyright 2022 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package linuxtls

import (
	"fmt"
	"os"
	"os/exec"

	"k8s.io/apimachinery/pkg/util/sets"
)

func OpenSSLRehash(dir string) (sets.Set[string], error) {
	filesBefore, err := listFilesInDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to list files in %q: %w", dir, err)
	}

	cmd := exec.Command("openssl", "rehash", dir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf(
			"failed to run openssl rehash on %q: %w (%s)",
			dir,
			err,
			string(output),
		)
	}

	filesAfter, err := listFilesInDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to list files in %q: %w", dir, err)
	}

	filesDiff := filesAfter.Difference(filesBefore)
	fmt.Println("New files:", filesDiff.UnsortedList())

	return filesDiff, nil
}

func listFilesInDir(dir string) (sets.Set[string], error) {
	dirEntries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	files := sets.New[string]()
	for _, d := range dirEntries {
		files = files.Insert(d.Name())
	}

	return files, nil
}
