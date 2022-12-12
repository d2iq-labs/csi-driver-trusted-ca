// Copyright 2022 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"

	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"

	"github.com/d2iq-labs/csi-driver-trusted-ca/cmd/app"
)

func main() {
	ctx := signals.SetupSignalHandler()
	cmd := app.NewCommand(ctx)

	if err := cmd.Execute(); err != nil {
		klog.ErrorS(err, "error running csi-driver-trusted-ca")
		os.Exit(1)
	}
}
