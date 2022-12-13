[38;5;227m[0m[38;5;227m─────────────────────────────────────────────────────────────────────────────────────────────────────────[0m
[38;5;227mmodified: test/e2e/suites/csi_driver/driver_test.go
[38;5;227m─────────────────────────────────────────────────────────────────────────────────────────────────────────[0m
[1;35m[1;35m@ test/e2e/suites/csi_driver/driver_test.go:230 @[1m[1m[38;5;146m var _ = Describe("Successful",[0m
			Expect(contents).To(Equal(string(secret.Data["registry-ca.pem"]) + "\n\n"))[m
		})[m
[m
[1;31m[1;31m		It("Test curl without specifying CA certs[m[1;31;48;5;52m[m[1;31m", func(ctx SpecContext) {[m[0m
[1;32m[1;32m		It("Test curl without specifying CA certs[m[1;32;48;5;22m on Alpine[m[1;32m", func(ctx SpecContext) {[m[0m
			pod := runTestPodInNewNamespace(ctx, kindClusterClient, "alpine")[m
[m
			stdout, stderr, err := kubernetes.ExecuteInPod([m
[1;35m[1;35m@ test/e2e/suites/csi_driver/driver_test.go:242 @[1m[1m[38;5;146m var _ = Describe("Successful",[0m
			)[m
			Expect(err).NotTo(HaveOccurred(), "stdout: %s, stderr: %s", stdout, stderr)[m
		})[m
[7m[1;32m [m
[1;32m[1;32m[m		[1;32mIt("Test curl without specifying CA certs on Debian", func(ctx SpecContext) {[m[0m
[1;32m[1;32m[m			[1;32mpod := runTestPodInNewNamespace(ctx, kindClusterClient, "debian")[m[0m
[7m[1;32m [m
[1;32m[1;32m[m			[1;32mstdout, stderr, err := kubernetes.ExecuteInPod([m[0m
[1;32m[1;32m[m				[1;32mctx,[m[0m
[1;32m[1;32m[m				[1;32mkindClusterClient,[m[0m
[1;32m[1;32m[m				[1;32mkindClusterRESTConfig,[m[0m
[1;32m[1;32m[m				[1;32mpod.Namespace, pod.Name, "container",[m[0m
[1;32m[1;32m[m				[1;32m"curl", "-fsSL", fmt.Sprintf("https://%s", e2eConfig.Registry.Address),[m[0m
[1;32m[1;32m[m			[1;32m)[m[0m
[1;32m[1;32m[m			[1;32mExpect(err).NotTo(HaveOccurred(), "stdout: %s, stderr: %s", stdout, stderr)[m[0m
[1;32m[1;32m[m		[1;32m})[m[0m
	},[m
)[m
