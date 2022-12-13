// Copyright 2022 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package manager

import (
	"testing"

	"github.com/go-logr/logr/testr"

	"github.com/d2iq-labs/csi-driver-trusted-ca/pkg/metadata"
	"github.com/d2iq-labs/csi-driver-trusted-ca/pkg/storage"
)

func newDefaultTestOptions(t *testing.T) Options {
	t.Helper()
	return defaultTestOptions(t, Options{})
}

func defaultTestOptions(t *testing.T, opts Options) Options {
	t.Helper()
	if opts.MetadataReader == nil {
		store := storage.NewMemoryFS()
		opts.MetadataReader = store
	}
	if opts.Log == nil {
		log := testr.New(t)
		opts.Log = &log
	}
	if opts.NodeID == "" {
		opts.NodeID = "test-node-id"
	}
	if opts.WriteCertificates == nil {
		opts.WriteCertificates = func(metadata.Metadata, map[string][]byte) error { return nil }
	}
	if opts.GetCertificates == nil {
		opts.GetCertificates = func(metadata.Metadata) (map[string][]byte, error) {
			return map[string][]byte{"a": []byte("b")}, nil
		}
	}
	return opts
}
