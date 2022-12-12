// Copyright 2022 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/d2iq-labs/csi-driver-trusted-ca/cmd/app/options"
	csiapi "github.com/d2iq-labs/csi-driver-trusted-ca/pkg/apis/v1alpha1"
	"github.com/d2iq-labs/csi-driver-trusted-ca/pkg/driver"
	"github.com/d2iq-labs/csi-driver-trusted-ca/pkg/manager"
	"github.com/d2iq-labs/csi-driver-trusted-ca/pkg/storage"
)

const (
	helpOutput = "Container Storage Interface driver to manage trusted CA certificates"
)

// NewCommand will return a new command instance for the trusted-ca CSI driver.
func NewCommand(ctx context.Context) *cobra.Command {
	opts := options.New()

	cmd := &cobra.Command{
		Use:   "csi-driver-trusted-ca",
		Short: helpOutput,
		Long:  helpOutput,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return opts.Complete()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			log := opts.Logr.WithName("main")
			log.Info("building driver")

			store, err := storage.NewFilesystem(opts.Logr.WithName("storage"), opts.DataRoot)
			if err != nil {
				return fmt.Errorf("failed to setup filesystem: %w", err)
			}
			store.FSGroupVolumeAttributeKey = csiapi.FSGroupKey

			mngrlog := opts.Logr.WithName("manager")
			d, err := driver.New(opts.Endpoint, opts.Logr.WithName("driver"), &driver.Options{
				DriverName:    opts.DriverName,
				DriverVersion: "v0.3.0",
				NodeID:        opts.NodeID,
				Store:         store,
				Manager: manager.NewManagerOrDie(manager.Options{
					MetadataReader:    store,
					Log:               &mngrlog,
					NodeID:            opts.NodeID,
					WriteCertificates: store.WriteFiles,
				}),
			})
			if err != nil {
				return errors.New("failed to setup driver: " + err.Error())
			}

			go func() {
				<-ctx.Done()
				log.Info("shutting down driver", "context", ctx.Err())
				d.Stop()
			}()

			log.Info("running driver")
			if err := d.Run(); err != nil {
				return errors.New("failed running driver: " + err.Error())
			}

			return nil
		},
	}

	opts = opts.Prepare(cmd)

	return cmd
}
