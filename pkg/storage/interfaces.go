// Copyright 2022 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package storage

import (
	"fmt"

	"github.com/d2iq-labs/csi-driver-trusted-ca/pkg/metadata"
)

var (
	// ErrNotFound is an error type that can be matched against with `errors.Is`
	// and indicates that no metadata is available.
	ErrNotFound = fmt.Errorf("not found")

	ErrInvalidJSON = fmt.Errorf("invalid JSON")
)

// All storage implementations must implement this interface.
type Interface interface {
	// PathForVolume returns the data path for the given volume.
	PathForVolume(volumeID string) string

	// RemoveVolume removes all metadata and data for a volume.
	// This is a destructive, irreversible operation.
	RemoveVolume(volumeID string) error

	MetadataReader
	MetadataWriter
	DataWriter
}

// MetadataReader allows read-only access to metadata about volumes.
type MetadataReader interface {
	// ReadMetadata will read the metadata for a single volumeID.
	ReadMetadata(volumeID string) (metadata.Metadata, error)

	// ListVolumes will return a list of all volumeIDs in the storage backend.
	// Used when the driver restarts to resume processing of existing data.
	ListVolumes() ([]string, error)
}

// MetadataWriter writes metadata files to a storage backend.
type MetadataWriter interface {
	// WriteMetadata will write the metadata file for the given volume.
	// If the directory for this volume does not exist, it will return an error.
	WriteMetadata(volumeID string, meta metadata.Metadata) error

	// RegisterMetadata will create a directory for the given metadata and, if
	// the metadata file does not already exist or volume context has changed,
	// persist the given metadata file.
	// It will return true if the metadata file has been written, false
	// otherwise.
	RegisterMetadata(meta metadata.Metadata) (bool, error)
}

// DataWriter is used to write data (e.g. certificate and private keys) to the
// storage backend.
type DataWriter interface {
	WriteFiles(meta metadata.Metadata, files map[string][]byte) error
}
