// Copyright 2022 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

const (
	FSGroupKey = "trusted-ca.csi.labs.d2iq.com/fs-group"
)

const (
	// Well-known attribute keys that are present in the volume context, passed
	// from the Kubelet during PublishVolume calls.
	K8sVolumeContextKeyPodName      = "csi.storage.k8s.io/pod.name"
	K8sVolumeContextKeyPodNamespace = "csi.storage.k8s.io/pod.namespace"
	K8sVolumeContextKeyPodUID       = "csi.storage.k8s.io/pod.uid"
)
