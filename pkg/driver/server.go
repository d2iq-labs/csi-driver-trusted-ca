// Copyright 2022 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package driver

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/go-logr/logr"
	"github.com/kubernetes-csi/csi-lib-utils/protosanitizer"
	"google.golang.org/grpc"
)

type GRPCServer struct {
	server *grpc.Server
	lis    net.Listener
}

func NewGRPCServer(
	endpoint string,
	log logr.Logger,
	ids csi.IdentityServer,
	cs csi.ControllerServer,
	ns csi.NodeServer,
) (*GRPCServer, error) {
	proto, addr, err := parseEndpoint(endpoint)
	if err != nil {
		return nil, err
	}

	if proto == "unix" {
		addr = "/" + addr
		if err := os.Remove(addr); err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("failed to remove %q: %w", addr, err)
		}
	}

	listener, err := net.Listen(proto, addr)
	if err != nil {
		return nil, err
	}

	return NewGRPCServerWithListener(listener, log, ids, cs, ns), nil
}

func NewGRPCServerWithListener(
	lis net.Listener,
	log logr.Logger,
	ids csi.IdentityServer,
	cs csi.ControllerServer,
	ns csi.NodeServer,
) *GRPCServer {
	opts := []grpc.ServerOption{
		grpc.UnaryInterceptor(loggingInterceptor(log)),
	}
	server := grpc.NewServer(opts...)

	if ids != nil {
		csi.RegisterIdentityServer(server, ids)
	}
	if cs != nil {
		csi.RegisterControllerServer(server, cs)
	}
	if ns != nil {
		csi.RegisterNodeServer(server, ns)
	}

	return &GRPCServer{
		server: server,
		lis:    lis,
	}
}

func (s *GRPCServer) Run() error {
	return s.server.Serve(s.lis)
}

func (s *GRPCServer) Stop() {
	s.server.GracefulStop()
}

func (s *GRPCServer) ForceStop() {
	s.server.Stop()
}

func parseEndpoint(ep string) (proto, host string, err error) {
	if strings.HasPrefix(strings.ToLower(ep), "unix://") ||
		strings.HasPrefix(strings.ToLower(ep), "tcp://") {
		s := strings.SplitN(ep, "://", 2)
		if s[1] != "" {
			return s[0], s[1], nil
		}
	}
	return "", "", fmt.Errorf("invalid endpoint: %v", ep)
}

func loggingInterceptor(log logr.Logger) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler,
	) (interface{}, error) {
		log := log.WithValues(
			"rpc_method",
			info.FullMethod,
			"request",
			protosanitizer.StripSecrets(req),
		)
		log.V(3).Info("handling request")
		resp, err := handler(ctx, req)
		if err != nil {
			log.Error(err, "failed processing request")
		} else {
			log.V(5).Info("request completed", "response", protosanitizer.StripSecrets(resp))
		}
		return resp, err
	}
}
