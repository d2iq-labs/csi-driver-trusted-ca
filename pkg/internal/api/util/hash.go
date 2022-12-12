// Copyright 2022 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package util

import (
	"fmt"
	"hash/fnv"

	"k8s.io/apimachinery/pkg/util/rand"
)

// HashIdentifier is the function used to hash a Node ID or Volume ID for use
// as a label value on created CertificateRequest resources.
func HashIdentifier(s string) string {
	hf := fnv.New32()
	hf.Write([]byte(s))
	return rand.SafeEncodeString(fmt.Sprint(hf.Sum32()))
}
