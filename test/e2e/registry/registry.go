// Copyright 2022 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"context"
	"fmt"
	"net"
	"path/filepath"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/pkg/namesgenerator"
	"github.com/docker/go-connections/nat"
	"go.uber.org/multierr"

	"github.com/d2iq-labs/csi-driver-trusted-ca/test/e2e/docker"
	"github.com/d2iq-labs/csi-driver-trusted-ca/test/e2e/env"
	"github.com/d2iq-labs/csi-driver-trusted-ca/test/e2e/seedrng"
	"github.com/d2iq-labs/csi-driver-trusted-ca/test/e2e/tls"
)

type registryOptions struct {
	image         string
	dockerNetwork string
}

type Opt func(*registryOptions)

func WithImage(image string) Opt {
	return func(ro *registryOptions) { ro.image = image }
}

func WithDockerNetwork(network string) Opt {
	return func(ro *registryOptions) { ro.dockerNetwork = network }
}

func defaultRegistryOptions() registryOptions {
	return registryOptions{
		image:         "registry:2",
		dockerNetwork: "kind",
	}
}

type Registry struct {
	cleanup func(context.Context) error

	address         string
	hostPortAddress string
	caCertFile      string
}

func (r *Registry) Address() string {
	return r.address
}

func (r *Registry) HostPortAddress() string {
	return r.hostPortAddress
}

func (r *Registry) CACertFile() string {
	return r.caCertFile
}

func (r *Registry) Delete(ctx context.Context) error {
	return r.cleanup(ctx)
}

func NewRegistry(
	ctx context.Context,
	dir string,
	opts ...Opt,
) (*Registry, error) {
	seedrng.EnsureSeeded()

	rOpt := defaultRegistryOptions()
	for _, o := range opts {
		o(&rOpt)
	}

	containerName := strings.ReplaceAll(namesgenerator.GetRandomName(0), "_", "-")

	if err := tls.GenerateCertificates(dir, containerName); err != nil {
		return nil, fmt.Errorf("failed to generate registry certificates: %w", err)
	}

	containerCfg := container.Config{
		Image:        rOpt.image,
		ExposedPorts: nat.PortSet{nat.Port("5000"): struct{}{}},
		Env: []string{
			"REGISTRY_HTTP_TLS_CERTIFICATE=/certs/tls.crt",
			"REGISTRY_HTTP_TLS_KEY=/certs/tls.key",
		},
	}
	hostCfg := container.HostConfig{
		AutoRemove:   true,
		NetworkMode:  container.NetworkMode(rOpt.dockerNetwork),
		PortBindings: nat.PortMap{nat.Port("5000"): []nat.PortBinding{{HostIP: "127.0.0.1"}}},
		Mounts: []mount.Mount{{
			Type:     mount.TypeBind,
			Source:   dir,
			Target:   "/certs",
			ReadOnly: true,
		}},
	}

	containerInspect, err := docker.RunContainerInBackground(
		ctx,
		containerName,
		&containerCfg,
		&hostCfg,
		env.DockerHubUsername(),
		env.DockerHubPassword(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to run registry container: %w", err)
	}

	publishedPort, ok := containerInspect.NetworkSettings.Ports[nat.Port("5000/tcp")]
	if !ok {
		if deleteErr := docker.ForceDeleteContainer(ctx, containerInspect.ID); deleteErr != nil {
			err = multierr.Combine(err, deleteErr)
		}
		return nil, fmt.Errorf("failed to get localhost registry port: %w", err)
	}

	r := &Registry{
		cleanup: func(ctx context.Context) error { return docker.ForceDeleteContainer(ctx, containerInspect.ID) },

		address:         net.JoinHostPort(containerName, "5000"),
		hostPortAddress: net.JoinHostPort(publishedPort[0].HostIP, publishedPort[0].HostPort),
		caCertFile:      filepath.Join(dir, "ca.crt"),
	}

	return r, nil
}
