// Copyright 2022 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package kubernetes

import (
	"bytes"
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/kubectl/pkg/scheme"
)

func ExecuteInPod(
	ctx context.Context,
	c kubernetes.Interface, restCfg *rest.Config,
	podNamespace, podName, container string,
	command ...string,
) (stdout, stderr string, err error) {
	buf := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	request := c.CoreV1().RESTClient().
		Post().
		Namespace(podNamespace).
		Resource("pods").
		Name(podName).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Command: command,
			Stdin:   false,
			Stdout:  true,
			Stderr:  true,
			TTY:     false,
		}, scheme.ParameterCodec)
	exec, err := remotecommand.NewSPDYExecutor(restCfg, "POST", request.URL())
	if err != nil {
		return "", "", fmt.Errorf(
			"%w Failed executing command %s on %v/%v",
			err,
			command,
			podNamespace,
			podName,
		)
	}
	err = exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdout: buf,
		Stderr: errBuf,
	})
	if err != nil {
		return "", "", fmt.Errorf(
			"%w Failed executing command %s on %v/%v",
			err,
			command,
			podNamespace,
			podName,
		)
	}

	return buf.String(), errBuf.String(), nil
}
