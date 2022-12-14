// Copyright 2022 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/containerd/containerd/remotes"
	"github.com/klauspost/compress/gzip"
	ocispecv1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	"oras.land/oras-go/pkg/auth"
	dockerauth "oras.land/oras-go/pkg/auth/docker"
	"oras.land/oras-go/pkg/content"
	"oras.land/oras-go/pkg/oras"
	registryauth "oras.land/oras-go/pkg/registry/remote/auth"
)

const userAgent = "csi-driver-trusted-ca/v1alpha1"

type (
	// Client works with OCI-compliant registries.
	Client struct {
		debug       bool
		enableCache bool
		// path to repository config file e.g. ~/.docker/config.json
		credentialsFile    string
		out                io.Writer
		authorizer         auth.Client
		registryAuthorizer *registryauth.Client
		resolver           remotes.Resolver
	}

	// ClientOption allows specifying various settings configurable by the user for overriding the defaults
	// used when creating a new default client.
	ClientOption func(*Client)
)

// NewClient returns a new registry client with config.
func NewClient(options ...ClientOption) (*Client, error) {
	client := &Client{
		out: io.Discard,
	}
	for _, option := range options {
		option(client)
	}
	if client.authorizer == nil {
		authClient, err := dockerauth.NewClient()
		if err != nil {
			return nil, err
		}
		client.authorizer = authClient
	}
	if client.resolver == nil {
		headers := http.Header{}
		headers.Set("User-Agent", userAgent)
		opts := []auth.ResolverOption{auth.WithResolverHeaders(headers)}
		resolver, err := client.authorizer.ResolverWithOpts(opts...)
		if err != nil {
			return nil, err
		}
		client.resolver = resolver
	}

	// allocate a cache if option is set
	var cache registryauth.Cache
	if client.enableCache {
		cache = registryauth.DefaultCache
	}
	if client.registryAuthorizer == nil {
		client.registryAuthorizer = &registryauth.Client{
			Header: http.Header{
				"User-Agent": {userAgent},
			},
			Cache: cache,
			Credential: func(ctx context.Context, reg string) (registryauth.Credential, error) {
				dockerClient, ok := client.authorizer.(*dockerauth.Client)
				if !ok {
					return registryauth.EmptyCredential, errors.New(
						"unable to obtain docker client",
					)
				}

				username, password, err := dockerClient.Credential(reg)
				if err != nil {
					return registryauth.EmptyCredential, errors.New(
						"unable to retrieve credentials",
					)
				}

				// A blank returned username and password value is a bearer token
				if username == "" && password != "" {
					return registryauth.Credential{
						RefreshToken: password,
					}, nil
				}

				return registryauth.Credential{
					Username: username,
					Password: password,
				}, nil
			},
		}
	}
	return client, nil
}

// ClientOptDebug returns a function that sets the debug setting on client options set.
func ClientOptDebug(debug bool) ClientOption {
	return func(client *Client) {
		client.debug = debug
	}
}

// ClientOptEnableCache returns a function that sets the enableCache setting on a client options set.
func ClientOptEnableCache(enableCache bool) ClientOption {
	return func(client *Client) {
		client.enableCache = enableCache
	}
}

// ClientOptWriter returns a function that sets the writer setting on client options set.
func ClientOptWriter(out io.Writer) ClientOption {
	return func(client *Client) {
		client.out = out
	}
}

// ClientOptCredentialsFile returns a function that sets the credentialsFile setting on a client options set.
func ClientOptCredentialsFile(credentialsFile string) ClientOption {
	return func(client *Client) {
		client.credentialsFile = credentialsFile
	}
}

// ClientOptEnableCache returns a function that sets the enableCache setting on a client options set.
func ClientOptResolver(resolver remotes.Resolver) ClientOption {
	return func(client *Client) {
		client.resolver = resolver
	}
}

type (
	// LoginOption allows specifying various settings on login.
	LoginOption func(*loginOperation)

	loginOperation struct {
		username string
		password string
		insecure bool
	}
)

// Login logs into a registry.
func (c *Client) Login(ctx context.Context, host string, options ...LoginOption) error {
	operation := &loginOperation{}
	for _, option := range options {
		option(operation)
	}
	authorizerLoginOpts := []auth.LoginOption{
		auth.WithLoginContext(ctx),
		auth.WithLoginHostname(host),
		auth.WithLoginUsername(operation.username),
		auth.WithLoginSecret(operation.password),
		auth.WithLoginUserAgent(userAgent),
	}
	if operation.insecure {
		authorizerLoginOpts = append(authorizerLoginOpts, auth.WithLoginInsecure())
	}
	if err := c.authorizer.LoginWithOpts(authorizerLoginOpts...); err != nil {
		return err
	}
	fmt.Fprintln(c.out, "Login Succeeded")
	return nil
}

// LoginOptBasicAuth returns a function that sets the username/password settings on login.
func LoginOptBasicAuth(username, password string) LoginOption {
	return func(operation *loginOperation) {
		operation.username = username
		operation.password = password
	}
}

// LoginOptInsecure returns a function that sets the insecure setting on login.
func LoginOptInsecure(insecure bool) LoginOption {
	return func(operation *loginOperation) {
		operation.insecure = insecure
	}
}

type (
	// LogoutOption allows specifying various settings on logout.
	LogoutOption func(*logoutOperation)

	logoutOperation struct{}
)

// Logout logs out of a registry.
func (c *Client) Logout(ctx context.Context, host string, opts ...LogoutOption) error {
	operation := &logoutOperation{}
	for _, opt := range opts {
		opt(operation)
	}
	if err := c.authorizer.Logout(ctx, host); err != nil {
		return err
	}
	fmt.Fprintf(c.out, "Removing login credentials for %s\n", host)
	return nil
}

// Pull downloads a chart from a registry.
func (c *Client) Pull(
	ctx context.Context,
	ref string,
) (r *tar.Reader, closeFunc func() error, err error) {
	parsedRef, err := parseReference(ref)
	if err != nil {
		return nil, nil, err
	}

	memoryStore := content.NewMemory()

	var descriptors, layers []ocispecv1.Descriptor
	registryStore := content.Registry{Resolver: c.resolver}

	manifest, err := oras.Copy(ctx, registryStore, parsedRef.String(), memoryStore, "",
		oras.WithPullEmptyNameAllowed(),
		oras.WithLayerDescriptors(func(l []ocispecv1.Descriptor) {
			layers = l
		}))
	if err != nil {
		return nil, nil, err
	}

	descriptors = append(descriptors, manifest)
	descriptors = append(descriptors, layers...)

	numDescriptors := len(descriptors)
	if numDescriptors < 1 {
		return nil, nil, fmt.Errorf(
			"manifest does not contain minimum number of descriptors (%d), descriptors found: %d",
			1,
			numDescriptors,
		)
	}
	var tarballDescriptor *ocispecv1.Descriptor
	for _, descriptor := range descriptors {
		d := descriptor
		if d.MediaType == ocispecv1.MediaTypeImageLayer ||
			d.MediaType == ocispecv1.MediaTypeImageLayerGzip {
			tarballDescriptor = &d
			break
		}
	}
	if tarballDescriptor == nil {
		return nil, nil, fmt.Errorf(
			"could not load config with one of mediatypes %q",
			[]string{ocispecv1.MediaTypeImageLayer, ocispecv1.MediaTypeImageLayerGzip},
		)
	}
	dataReader, err := memoryStore.Fetch(ctx, *tarballDescriptor)
	if err != nil {
		return nil, nil, fmt.Errorf(
			"unable to retrieve blob with digest %s: %w",
			tarballDescriptor.Digest,
			err,
		)
	}
	switch tarballDescriptor.MediaType {
	case ocispecv1.MediaTypeImageLayer:
		r = tar.NewReader(dataReader)
	case ocispecv1.MediaTypeImageLayerGzip:
		gzipReader, err := gzip.NewReader(dataReader)
		if err != nil {
			if closeErr := dataReader.Close(); closeErr != nil {
				err = multierr.Combine(err, closeErr)
			}
			return nil, nil, err
		}
		r = tar.NewReader(gzipReader)
	}

	return r, dataReader.Close, nil
}
