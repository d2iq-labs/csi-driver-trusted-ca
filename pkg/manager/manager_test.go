// Copyright 2022 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package manager

import (
	"context"
	"testing"
	"time"

	"github.com/d2iq-labs/csi-driver-trusted-ca/pkg/metadata"
	"github.com/d2iq-labs/csi-driver-trusted-ca/pkg/storage"
)

func TestManager_ManageVolumeImmediate_issueOnceAndSucceed(t *testing.T) {
	ctx := context.Background()

	opts := newDefaultTestOptions(t)
	m, err := NewManager(opts)
	if err != nil {
		t.Fatal(err)
	}

	// Register a new volume with the metadata store
	store := opts.MetadataReader.(storage.Interface)
	meta := metadata.Metadata{
		VolumeID:   "vol-id",
		TargetPath: "/fake/path",
	}
	store.RegisterMetadata(meta)
	// Ensure we stop managing the volume after the test
	defer func() {
		store.RemoveVolume(meta.VolumeID)
		m.UnmanageVolume(meta.VolumeID)
	}()

	// Attempt to issue the certificate & put it under management
	managed, err := m.ManageVolumeImmediate(ctx, meta.VolumeID)
	if !managed {
		t.Errorf("expected management to have started, but it did not")
	}
	if err != nil {
		t.Errorf("expected no error from ManageVolumeImmediate but got: %v", err)
	}

	// Assert the certificate is under management
	if !m.IsVolumeReady(meta.VolumeID) {
		t.Errorf("expected volume to be marked as Ready but it is not")
	}
}

func TestManager_ResumesManagementOfExistingVolumes(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()

	store := storage.NewMemoryFS()
	opts := defaultTestOptions(t, Options{MetadataReader: store})

	m, err := NewManager(opts)
	if err != nil {
		t.Fatal(err)
	}

	// Register a new volume with the metadata store
	meta := metadata.Metadata{
		VolumeID:   "vol-id",
		TargetPath: "/fake/path",
	}
	_, err = store.RegisterMetadata(meta)
	if err != nil {
		t.Fatal(err)
	}

	// Attempt to issue the certificate & put it under management
	managed, err := m.ManageVolumeImmediate(ctx, meta.VolumeID)
	if !managed {
		t.Fatalf("expected volume to be managed")
	}
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Shutdown the manager
	m.Stop()

	m, err = NewManager(opts)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Stop()

	if !m.IsVolumeReady(meta.VolumeID) {
		t.Errorf("expected volume to be monitored & ready but it is not")
	}
}

func TestManager_ManageVolume_beginsManagingAndProceedsIfNotReady(t *testing.T) {
	opts := newDefaultTestOptions(t)
	m, err := NewManager(opts)
	if err != nil {
		t.Fatal(err)
	}

	// Register a new volume with the metadata store
	store := opts.MetadataReader.(storage.Interface)
	meta := metadata.Metadata{
		VolumeID:   "vol-id",
		TargetPath: "/fake/path",
	}
	store.RegisterMetadata(meta)
	// Ensure we stop managing the volume after the test
	defer func() {
		store.RemoveVolume(meta.VolumeID)
		m.UnmanageVolume(meta.VolumeID)
	}()

	// Put the certificate under management
	managed := m.ManageVolume(meta.VolumeID)
	if !managed {
		t.Errorf("expected management to have started, but it did not")
	}

	if _, ok := m.managedVolumes[meta.VolumeID]; !ok {
		t.Errorf("expected volume to be part of managedVolumes map but it is not")
	}
}
