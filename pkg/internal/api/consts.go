// Copyright 2022 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package api

import csiapi "github.com/d2iq-labs/csi-driver-trusted-ca/pkg/apis/v1alpha1"

const (
	NodeIDHashLabelKey   = csiapi.DriverName + "/node-id-hash"
	VolumeIDHashLabelKey = csiapi.DriverName + "/volume-id-hash"
)
