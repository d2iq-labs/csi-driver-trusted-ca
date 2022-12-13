# Copyright 2022 D2iQ, Inc. All rights reserved.
# SPDX-License-Identifier: Apache-2.0

# syntax=docker/dockerfile:1

FROM alpine:3.17

RUN apk add openssl p11-kit

COPY csi-driver-trusted-ca /usr/local/bin/csi-driver-trusted-ca

ENTRYPOINT ["/usr/local/bin/csi-driver-trusted-ca"]
