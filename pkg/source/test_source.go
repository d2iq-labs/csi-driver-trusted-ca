// Copyright 2022 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package source

import (
	"context"

	"github.com/d2iq-labs/csi-driver-trusted-ca/pkg/metadata"
)

func newTestSource(_ string) (Source, error) {
	return testSource{}, nil
}

type testSource struct{}

func (testSource) GetFiles(_ context.Context, _ metadata.Metadata) (map[string][]byte, error) {
	return map[string][]byte{"a": []byte("b")}, nil
}
