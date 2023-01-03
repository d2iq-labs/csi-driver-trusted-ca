<!--
 Copyright 2022 D2iQ, Inc. All rights reserved.
 SPDX-License-Identifier: Apache-2.0
-->

# Trusted CAs CSI Driver

![GitHub](https://img.shields.io/github/license/d2iq-labs/csi-driver-trusted-ca?style=for-the-badge)
![GitHub Workflow Status](https://img.shields.io/github/actions/workflow/status/d2iq-labs/csi-driver-trusted-ca/checks.yml?branch=main&style=for-the-badge)

:warning: ***This project is an early prototype and subject to change. It is not yet suitable for production use.***

Ths project attempts to bring consistency in how trusted CA certificates are managed across containers.

Container images today have trusted CA certificates baked into them at image build time. This presents a problem because
you can never be sure that the same CA certificates are trusted across all containers.

Because of this, what we end up doing generally is supplying individual certificates mounted into containers at runtime,
and then configuring individual applications to use these custom CA certificates. This gets trickier because different
languages sometimes require the trusted certificates in different formats, e.g. Java keystores.

This projects attempts to solve this by mounting CA bundles, retrieved from configurable sources, into containers via
ephemeral CSI volumes to replace the system certificate store that is baked into the container image. This allows for
runtime configuration of trusted CA certificates rather than image build time, and removes the need for applications to
be configured individually to support custom CA certificates.

## Certificate bundle sources

The CSI driver can be configured to pull certificate bundles from various sources. These are then written to an
ephemeral volume, manipulated as required for the container image (e.g. concatenating individual certificate files into
a single bundle file, run `openssl rehash` on specified files, etc).

:information_source: Currently this project only supports a single source, but this will be extended in future to be
able to combine sources.

To configure the source, use the `--trusted-certs-source` flag, e.g.

```bash
csi-driver-trusted-ca --trusted-certs-source=<source>
```

Or specify it via the `trustedCertsSource` value when deploying via Helm.

### ConfigMap source

Use `configmap::<namespace>/<name>` (e.g. `configmap::mynamespace/cert-bundle`) to specify the configmap to use. Every
key in the configmap (from both `.data` and `.binaryData`) will be written to the CSI volume as an individual file.

### Secret source

Use `secret::<namespace>/<name>`  (e.g. `secret::mynamespace/cert-bundle`) to specify the secret to use. Every key in
the secret will be written to the CSI volume as an individual file.

### OCI source

Use `oci::<ociRef>` (e.g. `oci::myregistry/cert-bundle:v1`) to specify the OCI artifact to use. The OCI artifact must
have been pushed either with `application/vnd.oci.image.layer.v1.tar` or `application/vnd.oci.image.layer.v1.tar+gzip`
media type. The bundle will be unarchived (and decompressed if required) and files from the bundle will be written to
the CSI volume.

:information_source: To push an certificate bundle to an OCI registry, it's easiest to use the `oras` CLI. First create
the tarball via:

```bash
tar cvf certificate-bundle.tar cert.pem [cert2.pem ...]
```

Then push the artifact using `oras`:

```bash
oras push <YOUR_REGISTRY>/trusted-ca-certs:v1 certificate-bundle.tar:application/vnd.oci.image.layer.v1.tar
```

## Deployment

The container images and Helm chart are currently not released, but you can build them yourself via:

```shell
make release-snapshot
```

You then need to re-tag image:

```bash
docker tag d2iq-labs/csi-driver-trusted-ca:v0.1.0-dev <YOUR_REGISTRY>/d2iq-labs/csi-driver-trusted-ca:v0.1.0-dev
```

And push it to your registry:

```bash
docker push <YOUR_REGISTRY>/d2iq-labs/csi-driver-trusted-ca:v0.1.0-dev
```

You can then install or upgrade the CSI driver via Helm:

```bash
helm upgrade --install csi-driver-trusted-ca ./charts/csi-driver \
  --namespace kube-system \
  --set-string image.repository=<YOUR_REGISTRY>/d2iq-labs/csi-driver-trusted-ca \
  --set-string image.tag=v0.1.0-dev \
  --set-string trustedCertsSource=<SOURCE>
```
