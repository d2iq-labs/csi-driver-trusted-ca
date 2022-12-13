// Copyright 2022 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package manager

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/go-logr/logr"

	internalapiutil "github.com/d2iq-labs/csi-driver-trusted-ca/pkg/internal/api/util"
	"github.com/d2iq-labs/csi-driver-trusted-ca/pkg/storage"
)

// Options used to construct a Manager.
type Options struct {
	// Used the read metadata from the storage backend
	MetadataReader storage.MetadataReader

	// Logger used to write log messages
	Log *logr.Logger

	// NodeID is a unique identifier for the node.
	NodeID string

	GetCertificates GetCertificatesFunc

	WriteCertificates WriteCertificatesFunc
}

// NewManager constructs a new manager used to manage volumes containing
// certificate data.
// It will enumerate all volumes already persisted in the metadata store and
// resume managing them if any already exist.
func NewManager(opts Options) (*Manager, error) {
	if opts.Log == nil {
		return nil, errors.New("log must be set")
	}
	if opts.MetadataReader == nil {
		return nil, errors.New("metadataReader must be set")
	}
	if opts.GetCertificates == nil {
		return nil, errors.New("getCertificates must be set")
	}
	if opts.WriteCertificates == nil {
		return nil, errors.New("writeCertificates must be set")
	}
	if opts.NodeID == "" {
		return nil, errors.New("nodeID must be set")
	}
	nodeNameHash := internalapiutil.HashIdentifier(opts.NodeID)

	m := &Manager{
		metadataReader: opts.MetadataReader,
		log:            *opts.Log,

		managedVolumes: map[string]chan struct{}{},

		nodeNameHash: nodeNameHash,

		getCertificates: opts.GetCertificates,

		writeCertificates: opts.WriteCertificates,
	}

	vols, err := opts.MetadataReader.ListVolumes()
	if err != nil {
		return nil, fmt.Errorf("listing existing volumes: %w", err)
	}

	for _, vol := range vols {
		log := m.log.WithValues("volume_id", vol)
		_, err := opts.MetadataReader.ReadMetadata(vol)
		if err != nil {
			// This implies something has modified the state store whilst we are starting up
			// return the error and hope that next time we startup, nothing else changes the filesystem
			return nil, fmt.Errorf("reading existing volume metadata: %w", err)
		}
		log.Info("Registering existing data directory for management", "volume", vol)
		m.ManageVolume(vol)
	}

	return m, nil
}

func NewManagerOrDie(opts Options) *Manager {
	m, err := NewManager(opts)
	if err != nil {
		panic("failed to start manager: " + err.Error())
	}
	return m
}

// A Manager will manage trusted CA certificates in a storage backend.
// It is responsible for:
// * Retrieving trusted CA certificates
// * Persisting the certs back to the storage backend
//
// It also will trigger renewals of certificates when required.
type Manager struct {
	// used to read metadata from the store
	metadataReader storage.MetadataReader

	log logr.Logger

	lock sync.Mutex
	// global view of all volumes managed by this manager
	// the stored channel is used to stop management of the
	// volume
	managedVolumes map[string]chan struct{}

	// hash of the node name this driver is running on, used to label CertificateRequest
	// resources to allow the lister to be scoped to requests for this node only
	nodeNameHash string

	getCertificates GetCertificatesFunc

	writeCertificates WriteCertificatesFunc
}

// ManageVolumeImmediate will register a volume for management and immediately attempt to retrieve the trusted CA certs.
// Upon failure, it is the caller's responsibility to explicitly call `UnmanageVolume`.
func (m *Manager) ManageVolumeImmediate(
	ctx context.Context,
	volumeID string,
) (managed bool, err error) {
	if !m.manageVolumeIfNotManaged(volumeID) {
		return false, nil
	}

	meta, err := m.metadataReader.ReadMetadata(volumeID)
	if err != nil {
		return true, fmt.Errorf("reading metadata: %w", err)
	}

	files, err := m.getCertificates(ctx, meta)
	if err != nil {
		return true, err
	}

	if err := m.writeCertificates(meta, files); err != nil {
		return true, err
	}

	return true, nil
}

// manageVolumeIfNotManaged will ensure the named volume has been registered for management.
// It returns 'true' if the volume was not previously managed, and false if the volume was already managed.
func (m *Manager) manageVolumeIfNotManaged(volumeID string) (managed bool) {
	m.lock.Lock()
	defer m.lock.Unlock()
	log := m.log.WithValues("volume_id", volumeID)

	// if the volume is already managed, return early
	if _, managed := m.managedVolumes[volumeID]; managed {
		log.V(2).Info("Volume already registered for management")
		return false
	}

	// construct a new channel used to stop management of the volume
	stopCh := make(chan struct{})
	m.managedVolumes[volumeID] = stopCh

	return true
}

// ManageVolume will initiate management of data for the given volumeID. It will not wait for initial CA cert retrieval
// and instead rely on the update loop to retrieve the initial truested CA certificates.
// Callers can use `IsVolumeReady` to determine if a CA certificates have been successfully retrieved or not.
// Upon failure, it is the callers responsibility to call `UnmanageVolume`.
func (m *Manager) ManageVolume(volumeID string) (managed bool) {
	if managed := m.manageVolumeIfNotManaged(volumeID); !managed {
		return false
	}
	return true
}

func (m *Manager) UnmanageVolume(volumeID string) {
	m.lock.Lock()
	defer m.lock.Unlock()

	if stopCh, ok := m.managedVolumes[volumeID]; ok {
		close(stopCh)
		delete(m.managedVolumes, volumeID)
	}
}

func (m *Manager) IsVolumeReady(volumeID string) bool {
	m.lock.Lock()
	defer m.lock.Unlock()
	// a volume is not classed as Ready if it is not managed
	if _, managed := m.managedVolumes[volumeID]; !managed {
		m.log.V(2).Info("Volume is not yet managed")
		return false
	}
	m.log.V(2).Info("Volume is already managed")

	_, err := m.metadataReader.ReadMetadata(volumeID)
	if err != nil {
		m.log.Error(err, "failed to read metadata", "volume_id", volumeID)
		return false
	}

	return true
}

// Stop will stop management of all managed volumes.
func (m *Manager) Stop() {
	m.lock.Lock()
	defer m.lock.Unlock()
	for k, stopCh := range m.managedVolumes {
		close(stopCh)
		delete(m.managedVolumes, k)
	}
}
