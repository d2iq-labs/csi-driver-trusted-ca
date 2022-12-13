// Copyright 2022 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package source

import (
	"context"
	"fmt"
	"strings"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/d2iq-labs/csi-driver-trusted-ca/pkg/metadata"
)

var _ WithKubernetesClient = &configmapSource{}

func newConfigmapSource(cfg string) (Source, error) {
	cm := strings.Split(cfg, "/")

	if len(cm) != 2 {
		return nil, fmt.Errorf("invalid configmap source config: %s", cfg)
	}

	return &configmapSource{
		namespace: cm[0],
		name:      cm[1],
	}, nil
}

type configmapSource struct {
	namespace string
	name      string

	kc kubernetes.Interface
}

func (s *configmapSource) InjectKubernetesClient(kc kubernetes.Interface) {
	s.kc = kc
}

func (s *configmapSource) GetFiles(
	ctx context.Context,
	meta metadata.Metadata,
) (map[string][]byte, error) {
	cm, err := s.kc.CoreV1().ConfigMaps(s.namespace).Get(ctx, s.name, v1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to read files from configmap source: %w", err)
	}

	files := make(map[string][]byte, len(cm.Data)+len(cm.BinaryData))
	for k, v := range cm.Data {
		files[k] = []byte(v)
	}
	for k, v := range cm.BinaryData {
		files[k] = v
	}

	return files, nil
}
