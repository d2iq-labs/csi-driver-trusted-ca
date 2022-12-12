// Copyright 2022 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package options

import (
	"flag"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/rest"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/klog/v2"
	"k8s.io/klog/v2/klogr"
)

// Options are the main options for the driver. Populated via processing
// command line flags.
type Options struct {
	// logLevel is the verbosity level the driver will write logs at.
	logLevel string

	// kubeConfigFlags handles the Kubernetes authentication flags and builds a useable rest config.
	kubeConfigFlags *genericclioptions.ConfigFlags

	// NodeID is the name of the node which is hosting this driver instance.
	NodeID string

	// DriverName is the name of this CSI driver which will be shared with
	// the Kubelet.
	DriverName string

	// Endpoint is the endpoint that the driver will connect to the Kubelet.
	Endpoint string

	// DataRoot is the directory that the driver will write and mount volumes
	// from.
	DataRoot string

	// Logr is the shared base logger.
	Logr logr.Logger

	// RestConfig is the shared base rest config to connect to the Kubernetes
	// API.
	RestConfig *rest.Config
}

func New() *Options {
	return new(Options)
}

func (o *Options) Prepare(cmd *cobra.Command) *Options {
	o.addFlags(cmd)
	return o
}

func (o *Options) Complete() error {
	klog.InitFlags(nil)
	log := klogr.New()
	_ = flag.Set("v", o.logLevel)
	o.Logr = log

	var err error
	o.RestConfig, err = o.kubeConfigFlags.ToRESTConfig()
	if err != nil {
		return fmt.Errorf("failed to build kubernetes rest config: %s", err)
	}

	return nil
}

func (o *Options) addFlags(cmd *cobra.Command) {
	var nfs cliflag.NamedFlagSets

	o.addAppFlags(nfs.FlagSet("App"))
	o.kubeConfigFlags = genericclioptions.NewConfigFlags(true)
	o.kubeConfigFlags.AddFlags(nfs.FlagSet("Kubernetes"))
	_ = cmd.MarkPersistentFlagRequired("node-id")
	_ = cmd.MarkPersistentFlagRequired("endpoint")

	usageFmt := "Usage:\n  %s\n"
	cmd.SetUsageFunc(func(cmd *cobra.Command) error {
		fmt.Fprintf(cmd.OutOrStderr(), usageFmt, cmd.UseLine())
		cliflag.PrintSections(cmd.OutOrStderr(), nfs, 0)
		return nil
	})

	cmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		fmt.Fprintf(cmd.OutOrStdout(), "%s\n\n"+usageFmt, cmd.Long, cmd.UseLine())
		cliflag.PrintSections(cmd.OutOrStdout(), nfs, 0)
	})

	fs := cmd.Flags()
	for _, f := range nfs.FlagSets {
		fs.AddFlagSet(f)
	}
}

func (o *Options) addAppFlags(fs *pflag.FlagSet) {
	fs.StringVarP(&o.logLevel,
		"log-level", "v", "1",
		"Log level (1-5).")

	fs.StringVar(&o.NodeID, "node-id", "",
		"The name of the node which is hosting this driver instance.")

	fs.StringVar(&o.Endpoint, "endpoint", "",
		"The endpoint that the driver will connect to the Kubelet.")

	fs.StringVar(&o.DriverName, "driver-name", "trusted-ca.csi.labs.d2iq.com",
		"The name of this CSI driver which will be shared with the Kubelet.")

	fs.StringVar(&o.DataRoot, "data-root", "/csi-data-dir",
		"The directory that the driver will write and mount volumes from.")
}
