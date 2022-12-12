// Copyright 2022 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package storage

import (
	"encoding/json"
	"sync"

	apiequality "k8s.io/apimachinery/pkg/api/equality"

	"github.com/d2iq-labs/csi-driver-trusted-ca/pkg/metadata"
)

type MemoryFS struct {
	files map[string]map[string][]byte

	lock sync.Mutex
}

var _ Interface = &MemoryFS{}

func NewMemoryFS() *MemoryFS {
	return &MemoryFS{
		files: make(map[string]map[string][]byte),
	}
}

func (m *MemoryFS) PathForVolume(volumeID string) string {
	m.lock.Lock()
	defer m.lock.Unlock()
	return volumeID
}

func (m *MemoryFS) RemoveVolume(volumeID string) error {
	m.lock.Lock()
	defer m.lock.Unlock()
	delete(m.files, volumeID)
	return nil
}

func (m *MemoryFS) ReadMetadata(volumeID string) (metadata.Metadata, error) {
	m.lock.Lock()
	defer m.lock.Unlock()
	volMap, ok := m.files[volumeID]
	if !ok {
		return metadata.Metadata{}, ErrNotFound
	}
	metaFile, ok := volMap["metadata.json"]
	if !ok {
		return metadata.Metadata{}, ErrNotFound
	}
	meta := &metadata.Metadata{}
	if err := json.Unmarshal(metaFile, meta); err != nil {
		return metadata.Metadata{}, ErrInvalidJSON
	}
	return *meta, nil
}

func (m *MemoryFS) ListVolumes() ([]string, error) {
	m.lock.Lock()
	defer m.lock.Unlock()
	vols := make([]string, 0, len(m.files))
	for vol := range m.files {
		vols = append(vols, vol)
	}
	return vols, nil
}

func (m *MemoryFS) WriteMetadata(volumeID string, meta metadata.Metadata) error {
	m.lock.Lock()
	defer m.lock.Unlock()
	vol, ok := m.files[volumeID]
	if !ok {
		return ErrNotFound
	}
	metaJSON, _ := json.Marshal(meta)
	vol["metadata.json"] = metaJSON
	return nil
}

func (m *MemoryFS) RegisterMetadata(meta metadata.Metadata) (bool, error) {
	m.lock.Lock()
	defer m.lock.Unlock()
	vol, ok := m.files[meta.VolumeID]
	if !ok {
		vol = make(map[string][]byte)
		m.files[meta.VolumeID] = vol
	}

	existingMetaJSON, ok := vol["metadata.json"]
	if ok {
		var existingMeta metadata.Metadata
		if err := json.Unmarshal(existingMetaJSON, &existingMeta); err != nil {
			return false, err
		}

		// If the volume context hasn't changed in the existing metadata, no need to write
		if apiequality.Semantic.DeepEqual(existingMeta.VolumeContext, meta.VolumeContext) {
			return false, nil
		}
	}

	metaJSON, _ := json.Marshal(meta)
	vol["metadata.json"] = metaJSON

	return true, nil
}

func (m *MemoryFS) WriteFiles(meta metadata.Metadata, files map[string][]byte) error {
	m.lock.Lock()
	defer m.lock.Unlock()
	vol, ok := m.files[meta.VolumeID]
	if !ok {
		return ErrNotFound
	}
	for k, v := range files {
		vol[k] = v
	}
	return nil
}

func (m *MemoryFS) ReadFiles(volumeID string) (map[string][]byte, error) {
	m.lock.Lock()
	defer m.lock.Unlock()
	vol, ok := m.files[volumeID]
	if !ok {
		return nil, ErrNotFound
	}
	// make a copy of the map to ensure no races can occur
	cpy := make(map[string][]byte)
	for k, v := range vol {
		cpy[k] = v
	}
	return cpy, nil
}
