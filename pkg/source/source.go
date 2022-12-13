// Copyright 2022 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package source

import (
	"fmt"
	"regexp"

	"github.com/d2iq-labs/csi-driver-trusted-ca/pkg/metadata"
)

var (
	sourceRegexp = regexp.MustCompile(`^([A-Za-z0-9]+)::(.+)$`)

	getters = map[string]func(string) (Source, error){
		"test": newTestSource,
	}
)

type Source interface {
	GetFiles(metadata.Metadata) (map[string][]byte, error)
}

func New(src string) (Source, error) {
	getterName, getterConfig := getSource(src)
	getter, ok := getters[getterName]
	if ok {
		return getter(getterConfig)
	}

	return nil, fmt.Errorf("unsupported source: %s", src)
}

func newTestSource(_ string) (Source, error) {
	return testSource{}, nil
}

type testSource struct{}

func (testSource) GetFiles(_ metadata.Metadata) (map[string][]byte, error) {
	return map[string][]byte{"a": []byte("b")}, nil
}

func getSource(src string) (getterName, getterConfig string) {
	if ms := sourceRegexp.FindStringSubmatch(src); ms != nil {
		return ms[1], ms[2]
	}

	return "", ""
}
