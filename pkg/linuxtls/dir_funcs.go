// Copyright 2022 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package linuxtls

import (
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/d2iq-labs/csi-driver-trusted-ca/pkg/metadata"
)

func DirFuncsForVolume(meta metadata.Metadata) []func(dir string) (sets.Set[string], error) {
	return []func(dir string) (sets.Set[string], error){
		CreateCABundle,
		OpenSSLRehash,
	}
}
