<!--
 Copyright 2022 D2iQ, Inc. All rights reserved.
 SPDX-License-Identifier: Apache-2.0
 -->

# csi-driver-trusted-ca

![Version: v0.1.0](https://img.shields.io/badge/Version-v0.1.0-informational?style=flat-square)
![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square)
![AppVersion: v0.1.0](https://img.shields.io/badge/AppVersion-v0.1.0-informational?style=flat-square)

A Helm chart for csi-driver-trusted-ca

**Homepage:** <https://github.com/d2iq-labs/csi-driver-trusted-ca>

## Maintainers

| Name | Email | Url |
| ---- | ------ | --- |
| Jimmi Dyson | <jimmidyson@gmail.com> | <https://eng.d2iq.com> |

## Source Code

- <https://github.com/d2iq-labs/csi-driver-trusted-ca>

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| app.driver | object | `{"csiDataDir":"/tmp/csi-driver-trusted-ca","name":"trusted-ca.csi.labs.d2iq.com"}` | Options for CSI driver |
| app.driver.csiDataDir | string | `"/tmp/csi-driver-trusted-ca"` | Configures the hostPath directory that the driver will write and mount volumes from. |
| app.driver.name | string | `"trusted-ca.csi.labs.d2iq.com"` | Name of the driver which will be registered with Kubernetes. |
| app.kubeletRootDir | string | `"/var/lib/kubelet"` | Overrides path to root kubelet directory in case of a non-standard k8s install. |
| app.livenessProbe | object | `{"port":9809}` | Options for the liveness container. |
| app.livenessProbe.port | int | `9809` | The port that will expose the livness of the csi-driver |
| app.logLevel | int | `1` | Verbosity of csi-driver-trusted-ca logging. |
| image.pullPolicy | string | `"IfNotPresent"` | Kubernetes imagePullPolicy on csi-driver. |
| image.repository | string | `"mesosphere/csi-driver-trusted-ca"` | Target image repository. |
| image.tag | string | `"v0.1.0"` | Target image version tag. |
| imagePullSecrets | list | `[]` | Optional secrets used for pulling the csi-driver container image |
| livenessProbeImage.pullPolicy | string | `"IfNotPresent"` | Kubernetes imagePullPolicy on liveness probe. |
| livenessProbeImage.repository | string | `"k8s.gcr.io/sig-storage/livenessprobe"` | Target image repository. |
| livenessProbeImage.tag | string | `"v2.6.0"` | Target image version tag. |
| nodeDriverRegistrarImage.pullPolicy | string | `"IfNotPresent"` | Kubernetes imagePullPolicy on node-driver. |
| nodeDriverRegistrarImage.repository | string | `"k8s.gcr.io/sig-storage/csi-node-driver-registrar"` | Target image repository. |
| nodeDriverRegistrarImage.tag | string | `"v2.5.0"` | Target image version tag. |
| nodeSelector | object | `{}` |  |
| priorityClassName | string | `""` | Optional priority class to be used for the csi-driver pods. |
| resources | object | `{}` |  |
| tolerations | list | `[]` |  |
