// Copyright 2022 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package util

import (
	"bytes"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2"
)

const (
	maxFileNameLength = 255
	maxPathLength     = 4096
)

// AtomicWriter handles atomically projecting content for a set of files into
// a target directory.
//
// Note:
//
//  1. AtomicWriter reserves the set of pathnames starting with `..`.
//  2. AtomicWriter offers no concurrency guarantees and must be synchronized
//     by the caller.
//
// The visible files in this volume are symlinks to files in the writer's data
// directory.  Actual files are stored in a hidden timestamped directory which
// is symlinked to by the data directory. The timestamped directory and
// data directory symlink are created in the writer's target dir.  This scheme
// allows the files to be atomically updated by changing the target of the
// data directory symlink.
//
// Consumers of the target directory can monitor the ..data symlink using
// inotify or fanotify to receive events when the content in the volume is
// updated.
type AtomicWriter struct {
	targetDir  string
	logContext string
}

// FileProjection contains file Data and access Mode.
type FileProjection struct {
	Data   []byte
	Mode   int32
	FsUser *int64
}

// NewAtomicWriter creates a new AtomicWriter configured to write to the given
// target directory, or returns an error if the target directory does not exist.
func NewAtomicWriter(targetDir, logContext string) (*AtomicWriter, error) {
	_, err := os.Stat(targetDir)
	if os.IsNotExist(err) {
		return nil, err
	}

	return &AtomicWriter{targetDir: targetDir, logContext: logContext}, nil
}

const (
	dataDirName    = "..data"
	newDataDirName = "..data_tmp"
)

// Write does an atomic projection of the given payload into the writer's target
// directory.  Input paths must not begin with '..'.
//
// The Write algorithm is:
//
//  1. The payload is validated; if the payload is invalid, the function returns
//
//  2. The current timestamped directory is detected by reading the data directory
//     symlink
//
//  4. The data in the current timestamped directory is compared to the projected
//     data to determine if an update is required.
//
//  5. A new timestamped dir is created
//
//  6. The payload is written to the new timestamped directory
//
//  7. Specified functions are run to mutate files as necessary.
//
//  8. Symlinks and directory for new user-visible files are created (if needed).
//
//     For example, consider the files:
//     <target-dir>/podName
//     <target-dir>/user/labels
//     <target-dir>/k8s/annotations
//
//     The user visible files are symbolic links into the internal data directory:
//     <target-dir>/podName         -> ..data/podName
//     <target-dir>/usr -> ..data/usr
//     <target-dir>/k8s -> ..data/k8s
//
//     The data directory itself is a link to a timestamped directory with
//     the real data:
//     <target-dir>/..data          -> ..2016_02_01_15_04_05.12345678/
//
//  9. A symlink to the new timestamped directory ..data_tmp is created that will
//     become the new data directory
//
//  9. The old version of the volume is walked to determine whether any
//     portion of the payload was deleted and is still present on disk.
//
//  10. The new data directory symlink is renamed to the data directory; rename is atomic
//
// 11.  Old paths are removed from the user-visible portion of the target directory
// 12.  The previous timestamped directory is removed, if it exists.
func (w *AtomicWriter) Write(
	payload map[string]FileProjection,
	dirFuncs ...func(dir string) (newFiles sets.Set[string], err error),
) error {
	// (1)
	cleanPayload, err := validatePayload(payload)
	if err != nil {
		klog.Errorf("%s: invalid payload: %v", w.logContext, err)
		return err
	}

	// (2)
	dataDirPath := filepath.Join(w.targetDir, dataDirName)
	oldTSDir, err := os.Readlink(dataDirPath)
	if err != nil {
		if !os.IsNotExist(err) {
			klog.Errorf("%s: error reading link for data directory: %v", w.logContext, err)
			return err
		}
		// although Readlink() returns "" on err, don't be fragile by relying on it (since it's not specified in docs)
		// empty oldTSDir indicates that it didn't exist.
		oldTSDir = ""
	}
	oldTSPath := filepath.Join(w.targetDir, oldTSDir)

	// (4)
	tsDir, err := w.newTimestampDir()
	if err != nil {
		klog.V(4).Infof("%s: error creating new ts data directory: %v", w.logContext, err)
		return err
	}
	tsDirName := filepath.Base(tsDir)

	// (4)
	if err = w.writePayloadToDir(cleanPayload, tsDir); err != nil {
		klog.Errorf(
			"%s: error writing payload to ts data directory %s: %v",
			w.logContext,
			tsDir,
			err,
		)
		return err
	}
	klog.V(4).Infof("%s: performed write of new data to ts data directory: %s", w.logContext, tsDir)

	// (6)
	for _, f := range dirFuncs {
		newFiles, err := f(tsDir)
		if err != nil {
			klog.Errorf(
				"%s: error running specified post-write directory func on %s: %v",
				w.logContext,
				w.targetDir,
				err,
			)
			return fmt.Errorf("error running specified post-write directory func: %w", err)
		}
		for _, newFile := range newFiles.UnsortedList() {
			if err := validatePath(newFile); err != nil {
				return err
			}

			cleanPayload[filepath.Clean(newFile)] = FileProjection{}
		}
	}

	// (6)
	if err = w.createUserVisibleFiles(cleanPayload); err != nil {
		klog.Errorf("%s: error creating visible symlinks in %s: %v", w.logContext, w.targetDir, err)
		return err
	}

	// (7)
	newDataDirPath := filepath.Join(w.targetDir, newDataDirName)
	if err = os.Symlink(tsDirName, newDataDirPath); err != nil {
		os.RemoveAll(tsDir)
		klog.Errorf("%s: error creating symbolic link for atomic update: %v", w.logContext, err)
		return err
	}

	// (8)
	if runtime.GOOS == "windows" {
		os.Remove(dataDirPath)
		err = os.Symlink(tsDirName, dataDirPath)
		os.Remove(newDataDirPath)
	} else {
		err = os.Rename(newDataDirPath, dataDirPath)
	}
	if err != nil {
		os.Remove(newDataDirPath)
		os.RemoveAll(tsDir)
		klog.Errorf(
			"%s: error renaming symbolic link for data directory %s: %v",
			w.logContext,
			newDataDirPath,
			err,
		)
		return err
	}

	var pathsToRemove sets.Set[string]
	// if there was no old version, there's nothing to remove
	if oldTSDir != "" {
		// (9)
		pathsToRemove, err = w.pathsToRemove(cleanPayload, oldTSPath)
		if err != nil {
			klog.Errorf("%s: error determining user-visible files to remove: %v", w.logContext, err)
			return err
		}

		// (10)
		should, err := shouldWritePayload(cleanPayload, oldTSPath)
		switch {
		case err != nil:
			klog.Errorf(
				"%s: error determining whether payload should be written to disk: %v",
				w.logContext,
				err,
			)
			return err
		case !should && len(pathsToRemove) == 0:
			klog.V(4).
				Infof("%s: no update required for target directory %v", w.logContext, w.targetDir)
			return nil
		default:
			klog.V(4).Infof("%s: write required for target directory %v", w.logContext, w.targetDir)
		}
	}

	// (11)
	if err = w.removeUserVisiblePaths(pathsToRemove); err != nil {
		klog.Errorf("%s: error removing old visible symlinks: %v", w.logContext, err)
		return err
	}

	// (12)
	if len(oldTSDir) > 0 {
		if err = os.RemoveAll(oldTSPath); err != nil {
			klog.Errorf("%s: error removing old data directory %s: %v", w.logContext, oldTSDir, err)
			return err
		}
	}

	return nil
}

// validatePayload returns an error if any path in the payload returns a copy of the payload with the paths cleaned.
func validatePayload(payload map[string]FileProjection) (map[string]FileProjection, error) {
	cleanPayload := make(map[string]FileProjection)
	for k, content := range payload {
		if err := validatePath(k); err != nil {
			return nil, err
		}

		cleanPayload[filepath.Clean(k)] = content
	}

	return cleanPayload, nil
}

// validatePath validates a single path, returning an error if the path is
// invalid.  paths may not:
//
// 1. be absolute
// 2. contain '..' as an element
// 3. start with '..'
// 4. contain filenames larger than 255 characters
// 5. be longer than 4096 characters.
func validatePath(targetPath string) error {
	// TODO: somehow unify this with the similar api validation,
	// validateVolumeSourcePath; the error semantics are just different enough
	// from this that it was time-prohibitive trying to find the right
	// refactoring to re-use.
	if targetPath == "" {
		return fmt.Errorf("invalid path: must not be empty: %q", targetPath)
	}
	if path.IsAbs(targetPath) {
		return fmt.Errorf("invalid path: must be relative path: %s", targetPath)
	}

	if len(targetPath) > maxPathLength {
		return fmt.Errorf(
			"invalid path: must be less than or equal to %d characters",
			maxPathLength,
		)
	}

	items := strings.Split(targetPath, string(os.PathSeparator))
	for _, item := range items {
		if item == ".." {
			return fmt.Errorf("invalid path: must not contain '..': %s", targetPath)
		}
		if len(item) > maxFileNameLength {
			return fmt.Errorf(
				"invalid path: filenames must be less than or equal to %d characters",
				maxFileNameLength,
			)
		}
	}
	if strings.HasPrefix(items[0], "..") && len(items[0]) > 2 {
		return fmt.Errorf("invalid path: must not start with '..': %s", targetPath)
	}

	return nil
}

// shouldWritePayload returns whether the payload should be written to disk.
func shouldWritePayload(payload map[string]FileProjection, oldTSDir string) (bool, error) {
	for userVisiblePath, fileProjection := range payload {
		shouldWrite, err := shouldWriteFile(
			filepath.Join(oldTSDir, userVisiblePath),
			fileProjection.Data,
		)
		if err != nil {
			return false, err
		}

		if shouldWrite {
			return true, nil
		}
	}

	return false, nil
}

// shouldWriteFile returns whether a new version of a file should be written to disk.
func shouldWriteFile(fPath string, content []byte) (bool, error) {
	_, err := os.Lstat(fPath)
	if os.IsNotExist(err) {
		return true, nil
	}

	contentOnFs, err := os.ReadFile(fPath)
	if err != nil {
		return false, err
	}

	return !bytes.Equal(content, contentOnFs), nil
}

// pathsToRemove walks the current version of the data directory and
// determines which paths should be removed (if any) after the payload is
// written to the target directory.
func (w *AtomicWriter) pathsToRemove(
	payload map[string]FileProjection,
	oldTSDir string,
) (sets.Set[string], error) {
	paths := sets.New[string]()
	visitor := func(path string, info os.FileInfo, err error) error {
		relativePath := strings.TrimPrefix(path, oldTSDir)
		relativePath = strings.TrimPrefix(relativePath, string(os.PathSeparator))
		if relativePath == "" {
			return nil
		}

		paths.Insert(relativePath)
		return nil
	}

	err := filepath.Walk(oldTSDir, visitor)
	if os.IsNotExist(err) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	klog.V(5).Infof("%s: current paths:   %+v", w.targetDir, paths.UnsortedList())

	newPaths := sets.New[string]()
	for file := range payload {
		// add all subpaths for the payload to the set of new paths
		// to avoid attempting to remove non-empty dirs
		for subPath := file; subPath != ""; {
			newPaths.Insert(subPath)
			subPath, _ = filepath.Split(subPath)
			subPath = strings.TrimSuffix(subPath, string(os.PathSeparator))
		}
	}
	klog.V(5).Infof("%s: new paths:       %+v", w.targetDir, newPaths.UnsortedList())

	result := paths.Difference(newPaths)
	klog.V(5).Infof("%s: paths to remove: %+v", w.targetDir, result)

	return result, nil
}

// newTimestampDir creates a new timestamp directory.
func (w *AtomicWriter) newTimestampDir() (string, error) {
	tsDir, err := os.MkdirTemp(w.targetDir, time.Now().UTC().Format("..2006_01_02_15_04_05."))
	if err != nil {
		klog.Errorf("%s: unable to create new temp directory: %v", w.logContext, err)
		return "", err
	}

	// 0755 permissions are needed to allow 'group' and 'other' to recurse the
	// directory tree.  do a chmod here to ensure that permissions are set correctly
	// regardless of the process' umask.
	err = os.Chmod(tsDir, 0o755)
	if err != nil {
		klog.Errorf("%s: unable to set mode on new temp directory: %v", w.logContext, err)
		return "", err
	}

	return tsDir, nil
}

// writePayloadToDir writes the given payload to the given directory.  The
// directory must exist.
func (w *AtomicWriter) writePayloadToDir(payload map[string]FileProjection, dir string) error {
	for userVisiblePath, fileProjection := range payload {
		content := fileProjection.Data
		mode := os.FileMode(fileProjection.Mode)
		fullPath := filepath.Join(dir, userVisiblePath)
		baseDir, _ := filepath.Split(fullPath)

		if err := os.MkdirAll(baseDir, os.ModePerm); err != nil {
			klog.Errorf("%s: unable to create directory %s: %v", w.logContext, baseDir, err)
			return err
		}

		if err := os.WriteFile(fullPath, content, mode); err != nil {
			klog.Errorf(
				"%s: unable to write file %s with mode %v: %v",
				w.logContext,
				fullPath,
				mode,
				err,
			)
			return err
		}
		// Chmod is needed because os.WriteFile() ends up calling
		// open(2) to create the file, so the final mode used is "mode &
		// ~umask". But we want to make sure the specified mode is used
		// in the file no matter what the umask is.
		if err := os.Chmod(fullPath, mode); err != nil {
			klog.Errorf(
				"%s: unable to change file %s with mode %v: %v",
				w.logContext,
				fullPath,
				mode,
				err,
			)
			return err
		}

		if fileProjection.FsUser == nil {
			continue
		}
		if err := os.Chown(fullPath, int(*fileProjection.FsUser), -1); err != nil {
			klog.Errorf(
				"%s: unable to change file %s with owner %v: %v",
				w.logContext,
				fullPath,
				int(*fileProjection.FsUser),
				err,
			)
			return err
		}
	}

	return nil
}

// createUserVisibleFiles creates the relative symlinks for all the
// files configured in the payload. If the directory in a file path does not
// exist, it is created.
//
// Viz:
// For files: "bar", "foo/bar", "baz/bar", "foo/baz/blah"
// the following symlinks are created:
// bar -> ..data/bar
// foo -> ..data/foo
// baz -> ..data/baz.
func (w *AtomicWriter) createUserVisibleFiles(payload map[string]FileProjection) error {
	for userVisiblePath := range payload {
		slashpos := strings.Index(userVisiblePath, string(os.PathSeparator))
		if slashpos == -1 {
			slashpos = len(userVisiblePath)
		}
		linkname := userVisiblePath[:slashpos]
		_, err := os.Readlink(filepath.Join(w.targetDir, linkname))
		if err != nil && os.IsNotExist(err) {
			// The link into the data directory for this path doesn't exist; create it
			visibleFile := filepath.Join(w.targetDir, linkname)
			dataDirFile := filepath.Join(dataDirName, linkname)

			err = os.Symlink(dataDirFile, visibleFile)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// removeUserVisiblePaths removes the set of paths from the user-visible
// portion of the writer's target directory.
func (w *AtomicWriter) removeUserVisiblePaths(paths sets.Set[string]) error {
	ps := string(os.PathSeparator)
	var lasterr error
	for p := range paths {
		// only remove symlinks from the volume root directory (i.e. items that don't contain '/')
		if strings.Contains(p, ps) {
			continue
		}
		if err := os.Remove(filepath.Join(w.targetDir, p)); err != nil {
			klog.Errorf("%s: error pruning old user-visible path %s: %v", w.logContext, p, err)
			lasterr = err
		}
	}

	return lasterr
}
