// Copyright 2022 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package source

import (
	"context"
	"fmt"
	"regexp"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/d2iq-labs/csi-driver-trusted-ca/pkg/metadata"
)

var (
	sourceRegexp = regexp.MustCompile(`^([A-Za-z0-9]+)::(.+)$`)

	getters = map[string]func(string) (Source, error){
		"test":      newTestSource,
		"configmap": newConfigmapSource,
		"secret":    newSecretSource,
		"oci":       newOCISource,
	}
)

type Source interface {
	GetFiles(context.Context, metadata.Metadata) (map[string][]byte, error)
}

type WithRESTConfig interface {
	InjectRESTConfig(*rest.Config)
}

type WithKubernetesClient interface {
	InjectKubernetesClient(kubernetes.Interface)
}

func New(src string, restCfg *rest.Config) (Source, error) {
	getterName, getterConfig := getSource(src)
	getterFunc, ok := getters[getterName]
	if ok {
		getter, err := getterFunc(getterConfig)
		if err != nil {
			return nil, err
		}

		if gc, ok := getter.(WithKubernetesClient); ok {
			kc, err := kubernetes.NewForConfig(restCfg)
			if err != nil {
				return nil, err
			}

			gc.InjectKubernetesClient(kc)
		}
		if rc, ok := getter.(WithRESTConfig); ok {
			rc.InjectRESTConfig(restCfg)
		}

		return getter, nil
	}

	return nil, fmt.Errorf("unsupported source: %s", src)
}

func getSource(src string) (getterName, getterConfig string) {
	if ms := sourceRegexp.FindStringSubmatch(src); ms != nil {
		return ms[1], ms[2]
	}

	return "", ""
}
