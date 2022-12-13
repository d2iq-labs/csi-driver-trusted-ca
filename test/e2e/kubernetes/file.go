// Copyright 2022 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package kubernetes

import (
	"context"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func ReadFileFromPod(
	ctx context.Context,
	c kubernetes.Interface, restCfg *rest.Config,
	podNamespace, podName, container string,
	pathInContainer string,
) (contents, stderr string, err error) {
	command := []string{"sh", "-c", "cat " + pathInContainer}
	return ExecuteInPod(ctx, c, restCfg, podNamespace, podName, container, command...)
}
