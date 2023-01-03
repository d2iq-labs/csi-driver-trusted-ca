<!--
 Copyright 2022 D2iQ, Inc. All rights reserved.
 SPDX-License-Identifier: Apache-2.0
 -->

# Usage

[Helm](https://helm.sh) must be installed to use the charts.  Please refer to
Helm's [documentation](https://helm.sh/docs) to get started.

Once Helm has been set up correctly, add the repo as follows:

  helm repo add csi-driver-trusted-ca [https://d2iq-labs.github.io/csi-driver-trusted-ca](https://d2iq-labs.github.io/csi-driver-trusted-ca)

If you had already added this repo earlier, run `helm repo update` to retrieve
the latest versions of the packages.  You can then run `helm search repo
csi-driver-trusted-ca` to see the charts.

To install the csi-driver-trusted-ca chart:

    helm install my-csi-driver-trusted-ca csi-driver-trusted-ca/csi-driver-trusted-ca

To uninstall the chart:

    helm delete my-csi-driver-trusted-ca

For more details refer to the [README.md](https://github.com/d2iq-labs/csi-driver-trusted-ca/blob/main/README.md) in the main branch of the repository.
