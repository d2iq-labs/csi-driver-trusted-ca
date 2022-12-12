// Copyright 2022 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package manager

import "github.com/d2iq-labs/csi-driver-trusted-ca/pkg/metadata"

// WriteCertificatesFunc encodes & persists the output from a completed CertificateRequest
// into whatever storage backend is provided.
// The 'key' argument is as returned by the GeneratePrivateKeyFunc.
// The 'chain' and 'ca' arguments are PEM encoded and sourced directly from the
// CertificateRequest, without any attempt to parse or decode the bytes.
type WriteCertificatesFunc func(meta metadata.Metadata, cas map[string][]byte) error
