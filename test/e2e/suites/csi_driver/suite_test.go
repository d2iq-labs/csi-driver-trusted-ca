// Copyright 2022 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

//go:build e2e

package mirror_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/distribution/distribution/v3/reference"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/multierr"
	"helm.sh/helm/v3/pkg/cli/output"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/cli-utils/pkg/kstatus/status"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/kind/pkg/apis/config/v1alpha4"

	"github.com/d2iq-labs/csi-driver-trusted-ca/test/e2e/cluster"
	"github.com/d2iq-labs/csi-driver-trusted-ca/test/e2e/docker"
	"github.com/d2iq-labs/csi-driver-trusted-ca/test/e2e/env"
	"github.com/d2iq-labs/csi-driver-trusted-ca/test/e2e/goreleaser"
	"github.com/d2iq-labs/csi-driver-trusted-ca/test/e2e/helm"
	"github.com/d2iq-labs/csi-driver-trusted-ca/test/e2e/registry"
)

func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Mirror Suite")
}

type e2eSetupConfig struct {
	Registry   e2eRegistryConfig `json:"registry"`
	Kubeconfig string            `json:"kubeconfig"`
}

type e2eRegistryConfig struct {
	Address         string `json:"address"`
	HostPortAddress string `json:"hostPortAddress"`
	CACertFile      string `json:"caCertFile"`
}

var (
	kindClusterName       string
	kindClusterRESTConfig *rest.Config
	kindClusterClient     kubernetes.Interface
	e2eConfig             e2eSetupConfig
	artifacts             goreleaser.Artifacts
)

// func testdataPath(f string) string {
// 	return filepath.Join("testdata", f)
// }

var _ = SynchronizedBeforeSuite(
	func(ctx SpecContext) []byte {
		By("Parse goreleaser artifacts")
		var err error
		artifacts, err = goreleaser.ParseArtifactsFile(filepath.Join("..",
			"..",
			"..",
			"..",
			"dist",
			"artifacts.json",
		))
		Expect(err).NotTo(HaveOccurred())

		By("Starting Docker registry")
		mirrorRegistry, err := registry.NewRegistry(ctx, GinkgoT().TempDir())
		Expect(err).ToNot(HaveOccurred())
		DeferCleanup(mirrorRegistry.Delete, NodeTimeout(time.Minute))

		By("Starting KinD cluster")
		kindCluster, kcName, kubeconfig, err := cluster.NewKinDCluster(
			ctx,
			&v1alpha4.Cluster{
				Nodes: []v1alpha4.Node{{
					Role: v1alpha4.ControlPlaneRole,
					ExtraMounts: []v1alpha4.Mount{{
						HostPath:      mirrorRegistry.CACertFile(),
						ContainerPath: "/etc/containerd/mirror-registry-ca.pem",
						Readonly:      true,
					}},
				}},
				ContainerdConfigPatches: []string{
					fmt.Sprintf(
						`[plugins."io.containerd.grpc.v1.cri".registry.mirrors."docker.io"]
  endpoint = ["https://%[1]s"]
[plugins."io.containerd.grpc.v1.cri".registry.mirrors."k8s.gcr.io"]
  endpoint = ["https://%[1]s"]
[plugins."io.containerd.grpc.v1.cri".registry.mirrors."*"]
  endpoint = ["https://%[1]s"]
[plugins."io.containerd.grpc.v1.cri".registry.configs."%[1]s".tls]
  ca_file   = "/etc/containerd/mirror-registry-ca.pem"
`,
						mirrorRegistry.Address(),
					),
				},
			},
		)
		Expect(err).ToNot(HaveOccurred())
		DeferCleanup(kindCluster.Delete, NodeTimeout(time.Minute))
		kindClusterName = kcName

		e2eConfig = e2eSetupConfig{
			Registry: e2eRegistryConfig{
				Address:         mirrorRegistry.Address(),
				HostPortAddress: mirrorRegistry.HostPortAddress(),
				CACertFile:      mirrorRegistry.CACertFile(),
			},
			Kubeconfig: kubeconfig,
		}

		By("Pushing project Docker image to registry")
		img, ok := artifacts.SelectDockerImage(
			"d2iq-labs/csi-driver-trusted-ca",
			"linux",
			runtime.GOARCH,
		)
		Expect(ok).To(BeTrue())
		err = docker.RetagAndPushImage(
			ctx,
			img.Name,
			fmt.Sprintf("%s/%s", e2eConfig.Registry.HostPortAddress, img.Name),
			env.DockerHubUsername(),
			env.DockerHubPassword(),
		)
		Expect(err).NotTo(HaveOccurred())

		namedImg, err := reference.ParseNormalizedNamed(img.Name)
		Expect(err).NotTo(HaveOccurred())

		By("Installing trusted CA CSI driver daemonset")
		release, err := helm.InstallOrUpgrade(
			ctx,
			"csi-driver-trusted-ca",
			filepath.Join("..", "..", "..", "..", "charts", "csi-driver"),
			map[string]interface{}{
				"image": map[string]interface{}{
					"tag":        namedImg.(reference.NamedTagged).Tag(),
					"pullPolicy": corev1.PullAlways,
				},
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

		configBytes, _ := json.Marshal(e2eConfig)

		return configBytes
	},
	func(configBytes []byte) {
		Expect(json.Unmarshal(configBytes, &e2eConfig)).To(Succeed())

		var err error
		kindClusterRESTConfig, err = clientcmd.RESTConfigFromKubeConfig(
			[]byte(e2eConfig.Kubeconfig),
		)
		Expect(err).NotTo(HaveOccurred())
		kindClusterClient, err = kubernetes.NewForConfig(kindClusterRESTConfig)
		Expect(err).NotTo(HaveOccurred())
	},
	NodeTimeout(time.Minute*2), GracePeriod(time.Minute*2),
)

// func runPod(ctx context.Context, k8sClient kubernetes.Interface, image string) *corev1.Pod {
// 	pod, err := k8sClient.CoreV1().Pods(metav1.NamespaceDefault).
// 		Create(
// 			ctx,
// 			&corev1.Pod{
// 				ObjectMeta: metav1.ObjectMeta{GenerateName: "pod-"},
// 				Spec: corev1.PodSpec{
// 					Containers: []corev1.Container{{
// 						Name:            "container1",
// 						Image:           image,
// 						ImagePullPolicy: corev1.PullAlways,
// 					}},
// 				},
// 			},
// 			metav1.CreateOptions{},
// 		)
// 	Expect(err).NotTo(HaveOccurred())

// 	DeferCleanup(func(ctx SpecContext) {
// 		err := kindClusterClient.CoreV1().Pods(metav1.NamespaceDefault).
// 			Delete(ctx, pod.GetName(), *metav1.NewDeleteOptions(0))
// 		Expect(err).NotTo(HaveOccurred())
// 	}, NodeTimeout(time.Minute))

// 	return pod
// }

func objStatus(obj k8sruntime.Object, scheme *k8sruntime.Scheme) status.Status {
	if obj.GetObjectKind().GroupVersionKind().Group == "" {
		gvk, err := apiutil.GVKForObject(obj, scheme)
		Expect(err).NotTo(HaveOccurred())
		obj.GetObjectKind().SetGroupVersionKind(gvk)
	}

	m, err := k8sruntime.DefaultUnstructuredConverter.ToUnstructured(obj)
	Expect(err).NotTo(HaveOccurred())

	res, err := status.Compute(&unstructured.Unstructured{Object: m})
	Expect(err).NotTo(HaveOccurred())

	return res.Status
}
