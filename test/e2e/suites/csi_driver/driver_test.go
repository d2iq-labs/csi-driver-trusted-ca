// Copyright 2022 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

//go:build e2e

package csi_driver_test

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/multierr"
	"helm.sh/helm/v3/pkg/cli/output"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/cli-utils/pkg/kstatus/status"

	"github.com/d2iq-labs/csi-driver-trusted-ca/test/e2e/docker"
	"github.com/d2iq-labs/csi-driver-trusted-ca/test/e2e/env"
	"github.com/d2iq-labs/csi-driver-trusted-ca/test/e2e/helm"
	"github.com/d2iq-labs/csi-driver-trusted-ca/test/e2e/kubernetes"
)

var _ = Describe("Successful",
	Label("csi"),
	Ordered, Serial,
	func() {
		var (
			cm     *corev1.ConfigMap
			secret *corev1.Secret
		)

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

			By("Successfully create configmap with dummy data")
			var err error
			cm, err = kindClusterClient.CoreV1().ConfigMaps(corev1.NamespaceDefault).Create(
				ctx,
				&corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Namespace:    corev1.NamespaceDefault,
						GenerateName: "trusted-certs-",
					},
					Data: map[string]string{"c": "d"},
				},
				metav1.CreateOptions{},
			)
			Expect(err).NotTo(HaveOccurred())

			By("Successfully create secret with dummy data")
			secret, err = kindClusterClient.CoreV1().Secrets(corev1.NamespaceDefault).Create(
				ctx,
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Namespace:    corev1.NamespaceDefault,
						GenerateName: "trusted-certs-",
					},
					StringData: map[string]string{"e": "f"},
				},
				metav1.CreateOptions{},
			)
			Expect(err).NotTo(HaveOccurred())
		})

		It("CSI daemonset should be running on all nodes", func(ctx SpecContext) {
			nodes, err := kindClusterClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())

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

				if int(ds.Status.DesiredNumberScheduled) != len(nodes.Items) {
					return status.InProgressStatus
				}

				return objStatus(ds, scheme.Scheme)
			}, time.Minute, time.Second).WithContext(ctx).
				Should(Equal(status.CurrentStatus))
		})

		It("Mount populated CSI volume in pod with initial test data", func(ctx SpecContext) {
			pod := runTestPodInNewNamespace(ctx, kindClusterClient, "alpine")

			contents, stderr, err := kubernetes.ReadFileFromPod(
				ctx,
				kindClusterClient,
				kindClusterRESTConfig,
				pod.Namespace, pod.Name, "container",
				"/etc/ssl/certs/a",
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(stderr).To(BeEmpty())
			Expect(contents).To(Equal("b"))
		})

		reconfigureCSIDriver := func(ctx context.Context, src string) {
			release, err := helm.InstallOrUpgrade(
				ctx,
				"csi-driver-trusted-ca",
				filepath.Join("..", "..", "..", "..", "charts", "csi-driver"),
				map[string]interface{}{
					"trustedCertsSource": src,
				},
				e2eConfig.Kubeconfig,
				metav1.NamespaceSystem,
				GinkgoWriter.Printf,
				time.Minute,
			)
			var releaseYAML bytes.Buffer
			if encodeErr := output.EncodeYAML(&releaseYAML, release); encodeErr != nil {
				err = multierr.Combine(err, encodeErr)
			} else {
				AddReportEntry("helm release", ReportEntryVisibilityFailureOrVerbose, releaseYAML.String())
			}
			Expect(err).NotTo(HaveOccurred())

			nodes, err := kindClusterClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())

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

				if int(ds.Status.DesiredNumberScheduled) != len(nodes.Items) {
					return status.InProgressStatus
				}

				return objStatus(ds, scheme.Scheme)
			}, time.Minute, time.Second).WithContext(ctx).
				Should(Equal(status.CurrentStatus))
		}

		It(
			"Reconfigure trusted CA CSI driver daemonset to use configmap source",
			func(ctx SpecContext) {
				reconfigureCSIDriver(ctx, fmt.Sprintf("configmap::%s/%s", cm.Namespace, cm.Name))
			},
		)

		It("Mount populated CSI volume in pod with data from configmap", func(ctx SpecContext) {
			pod := runTestPodInNewNamespace(ctx, kindClusterClient, "alpine")

			contents, stderr, err := kubernetes.ReadFileFromPod(
				ctx,
				kindClusterClient,
				kindClusterRESTConfig,
				pod.Namespace, pod.Name, "container",
				"/etc/ssl/certs/c",
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(stderr).To(BeEmpty())
			Expect(contents).To(Equal("d"))
		})

		It(
			"Reconfigure trusted CA CSI driver daemonset to use configmap source",
			func(ctx SpecContext) {
				reconfigureCSIDriver(
					ctx,
					fmt.Sprintf("secret::%s/%s", secret.Namespace, secret.Name),
				)
			},
		)

		It("Mount populated CSI volume in pod with data from secret", func(ctx SpecContext) {
			pod := runTestPodInNewNamespace(ctx, kindClusterClient, "alpine")

			contents, stderr, err := kubernetes.ReadFileFromPod(
				ctx,
				kindClusterClient,
				kindClusterRESTConfig,
				pod.Namespace, pod.Name, "container",
				"/etc/ssl/certs/e",
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(stderr).To(BeEmpty())
			Expect(contents).To(Equal("f"))
		})
	},
)
