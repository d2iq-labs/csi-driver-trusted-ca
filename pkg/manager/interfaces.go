// Copyright 2022 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package manager

import "github.com/d2iq-labs/csi-driver-trusted-ca/pkg/metadata"

type GetCertificatesFunc func(meta metadata.Metadata) (map[string][]byte, error)

type WriteCertificatesFunc func(meta metadata.Metadata, cas map[string][]byte) error
