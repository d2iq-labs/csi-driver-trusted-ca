// Copyright 2022 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

//go:build e2e

package csi_driver_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/mholt/archiver/v3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	ocispecv1 "github.com/opencontainers/image-spec/specs-go/v1"
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
			for _, t := range []string{"ubi", "alpine", "debian", "golang"} {
				img := fmt.Sprintf("ghcr.io/d2iq-labs/csi-driver-trusted-ca-test:%s", t)
				err := docker.PushImageToDifferentRegistry(
					ctx,
					img,
					e2eConfig.Registry.HostPortAddress,
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

			By("Successfully create secret with data using registry CA certificate")
			caBytes, err := os.ReadFile(e2eConfig.Registry.CACertFile)
			Expect(err).NotTo(HaveOccurred())
			secret, err = kindClusterClient.CoreV1().Secrets(corev1.NamespaceDefault).Create(
				ctx,
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Namespace:    corev1.NamespaceDefault,
						GenerateName: "trusted-certs-",
					},
					Data: map[string][]byte{"registry-ca.pem": caBytes},
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

		Context("ConfigMap source", Label("configmap"), func() {
			It(
				"Reconfigure trusted CA CSI driver daemonset to use configmap source",
				func(ctx SpecContext) {
					reconfigureCSIDriver(
						ctx,
						fmt.Sprintf("configmap::%s/%s", cm.Namespace, cm.Name),
					)
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
		})

		Context("Secret source", Label("secret"), func() {
			It(
				"Reconfigure trusted CA CSI driver daemonset to use secret source",
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
					"/etc/ssl/certs/registry-ca.pem",
				)
				Expect(err).NotTo(HaveOccurred())
				Expect(stderr).To(BeEmpty())
				Expect(contents).To(Equal(string(secret.Data["registry-ca.pem"])))

				contents, stderr, err = kubernetes.ReadFileFromPod(
					ctx,
					kindClusterClient,
					kindClusterRESTConfig,
					pod.Namespace, pod.Name, "container",
					"/etc/ssl/certs/ca-certificates.crt",
				)
				Expect(err).NotTo(HaveOccurred())
				Expect(stderr).To(BeEmpty())
				Expect(contents).To(Equal(string(secret.Data["registry-ca.pem"]) + "\n\n"))
			})

			It("Test curl without specifying CA certs on Alpine", func(ctx SpecContext) {
				pod := runTestPodInNewNamespace(ctx, kindClusterClient, "alpine")

				stdout, stderr, err := kubernetes.ExecuteInPod(
					ctx,
					kindClusterClient,
					kindClusterRESTConfig,
					pod.Namespace, pod.Name, "container",
					"curl", "-fsSL", fmt.Sprintf("https://%s", e2eConfig.Registry.Address),
				)
				Expect(err).NotTo(HaveOccurred(), "stdout: %s, stderr: %s", stdout, stderr)
			})

			It("Test curl without specifying CA certs on Debian", func(ctx SpecContext) {
				pod := runTestPodInNewNamespace(ctx, kindClusterClient, "debian")

				stdout, stderr, err := kubernetes.ExecuteInPod(
					ctx,
					kindClusterClient,
					kindClusterRESTConfig,
					pod.Namespace, pod.Name, "container",
					"curl", "-fsSL", fmt.Sprintf("https://%s", e2eConfig.Registry.Address),
				)
				Expect(err).NotTo(HaveOccurred(), "stdout: %s, stderr: %s", stdout, stderr)
			})
		})

		Context("OCI source", Label("oci"), func() {
			It(
				"Push registry CA cert to OCI registry",
				func(ctx SpecContext) {
					tmpDir := GinkgoT().TempDir()
					tempTarArchive := filepath.Join(tmpDir, "ca-bundle.tar")
					Expect(
						archiver.Archive([]string{e2eConfig.Registry.CACertFile}, tempTarArchive),
					).To(Succeed())
					ociAddress := fmt.Sprintf(
						"%s/%s:%s",
						e2eConfig.Registry.HostPortAddress,
						"trusted-certs",
						"v1",
					)
					cmd := exec.CommandContext(
						ctx, "oras", "push", "--insecure", ociAddress,
						fmt.Sprintf("%s:%s", tempTarArchive, ocispecv1.MediaTypeImageLayer),
					)
					output, err := cmd.CombinedOutput()
					Expect(err).NotTo(HaveOccurred(), "output: %s", output)
				},
			)

			It(
				"Reconfigure trusted CA CSI driver daemonset to use OCI source",
				func(ctx SpecContext) {
					ociAddress := fmt.Sprintf(
						"%s/%s:%s",
						e2eConfig.Registry.Address,
						"trusted-certs",
						"v1",
					)
					reconfigureCSIDriver(
						ctx,
						fmt.Sprintf("oci::%s", ociAddress),
					)
				},
			)

			It(
				"Test curl without specifying CA certs on Alpine with certs from OCI",
				func(ctx SpecContext) {
					pod := runTestPodInNewNamespace(ctx, kindClusterClient, "alpine")

					stdout, stderr, err := kubernetes.ExecuteInPod(
						ctx,
						kindClusterClient,
						kindClusterRESTConfig,
						pod.Namespace, pod.Name, "container",
						"curl", "-fsSL", fmt.Sprintf("https://%s", e2eConfig.Registry.Address),
					)
					Expect(err).NotTo(HaveOccurred(), "stdout: %s, stderr: %s", stdout, stderr)
				},
			)

			It(
				"Test Go httpie-clone without specifying CA certs on Debian with certs from OCI",
				func(ctx SpecContext) {
					pod := runTestPodInNewNamespace(ctx, kindClusterClient, "golang")

					stdout, stderr, err := kubernetes.ExecuteInPod(
						ctx,
						kindClusterClient,
						kindClusterRESTConfig,
						pod.Namespace, pod.Name, "container",
						"/go/bin/ht", fmt.Sprintf("https://%s", e2eConfig.Registry.Address),
					)
					Expect(err).NotTo(HaveOccurred(), "stdout: %s, stderr: %s", stdout, stderr)
				},
			)
		})
	},
)
