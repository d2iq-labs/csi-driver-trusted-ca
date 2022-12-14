// Copyright 2022 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package source

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"

	"github.com/containerd/containerd/remotes/docker"
	"github.com/distribution/distribution/v3/reference"

	"github.com/d2iq-labs/csi-driver-trusted-ca/pkg/metadata"
	"github.com/d2iq-labs/csi-driver-trusted-ca/pkg/registry"
)

func newOCISource(cfg string) (Source, error) {
	ref, err := reference.ParseNormalizedNamed(cfg)
	if err != nil {
		return nil, fmt.Errorf("invalid OCI reference: %w", err)
	}
	return &ociSource{
		ref: ref,
	}, nil
}

type ociSource struct {
	ref reference.Named
}

func (s *ociSource) GetFiles(
	ctx context.Context,
	meta metadata.Metadata,
) (map[string][]byte, error) {
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true, //nolint:gosec // Yes, this is insecure - fix later.
	}
	transport := &http.Transport{
		TLSClientConfig: tlsConfig,
	}
	insecureClient := &http.Client{
		Transport: transport,
	}

	resolver := docker.NewResolver(
		docker.ResolverOptions{
			Hosts: docker.ConfigureDefaultRegistries(docker.WithClient(insecureClient)),
		},
	)

	ociClient, err := registry.NewClient(registry.ClientOptResolver(resolver))
	if err != nil {
		return nil, fmt.Errorf("failed to create OCI registry client: %w", err)
	}
	artifactReader, closeFn, err := ociClient.Pull(ctx, s.ref.String())
	if err != nil {
		return nil, fmt.Errorf("failed to read bundle from OCI registry: %w", err)
	}
	defer func() { _ = closeFn() }()

	files := map[string][]byte{}

	for {
		header, err := artifactReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read tarball: %w", err)
		}

		info := header.FileInfo()
		if info.IsDir() {
			continue
		}

		var buf bytes.Buffer
		_, err = io.Copy(&buf, artifactReader)
		if err != nil {
			return nil, fmt.Errorf("failed to read tarball: %w", err)
		}

		files[header.Name] = buf.Bytes()
	}

	return files, nil
}
