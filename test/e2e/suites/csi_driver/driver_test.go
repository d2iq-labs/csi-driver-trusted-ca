// Copyright 2022 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

//go:build e2e

package mirror_test

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/cli-utils/pkg/kstatus/status"

	"github.com/d2iq-labs/csi-driver-trusted-ca/test/e2e/docker"
	"github.com/d2iq-labs/csi-driver-trusted-ca/test/e2e/env"
)

var _ = Describe("Successful",
	Label("csi"),
	Ordered, Serial,
	func() {
		BeforeAll(func(ctx SpecContext) {
			By("Pushing e2e Docker images to registry")
			for _, t := range []string{"redhat", "alpine", "debian"} {
				img := fmt.Sprintf("d2iq-labs/csi-driver-trusted-ca-test:%s", t)
				err := docker.RetagAndPushImage(
					ctx,
					img,
					fmt.Sprintf("%s/%s", e2eConfig.Registry.HostPortAddress, img),
					env.DockerHubUsername(),
					env.DockerHubPassword(),
				)
				Expect(err).NotTo(HaveOccurred())
			}
		})

		It("CSI daemonset should be running", func(ctx SpecContext) {
			Eventually(func(ctx context.Context) status.Status {
				var err error
				ds, err := kindClusterClient.AppsV1().DaemonSets(metav1.NamespaceSystem).
					Get(ctx, "csi-driver-trusted-ca", metav1.GetOptions{})
				if err != nil {
					if errors.IsNotFound(err) {
						return status.NotFoundStatus
					}

					Expect(err).NotTo(HaveOccurred())
				}

				if ds.Status.DesiredNumberScheduled == 0 {
					return status.InProgressStatus
				}

				return objStatus(ds, scheme.Scheme)
			}, time.Minute, time.Second).WithContext(ctx).
				Should(Equal(status.CurrentStatus))
		})
	})
