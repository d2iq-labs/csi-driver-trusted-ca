// Copyright 2022 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package docker

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/distribution/distribution/v3/reference"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"go.uber.org/multierr"
)

var (
	clientFromEnv     *client.Client
	clientFromEnvOnce sync.Once
	errClientFromEnv  error
)

func ClientFromEnv() (*client.Client, error) {
	clientFromEnvOnce.Do(func() {
		var err error
		clientFromEnv, err = client.NewClientWithOpts(
			client.FromEnv,
			client.WithAPIVersionNegotiation(),
		)
		if err != nil {
			errClientFromEnv = fmt.Errorf(
				"failed to create Docker client from environment: %w",
				err,
			)
		}
	})

	return clientFromEnv, errClientFromEnv
}

//nolint:revive // Complex but only used in tests.
func RunContainerInBackground(
	ctx context.Context,
	containerName string,
	containerCfg *container.Config,
	hostCfg *container.HostConfig,
	pullUsername, pullPassword string,
) (types.ContainerJSON, error) {
	dClient, err := ClientFromEnv()
	if err != nil {
		return types.ContainerJSON{}, fmt.Errorf("failed to create Docker client: %w", err)
	}

	if hostCfg.NetworkMode.IsUserDefined() {
		_, err = dClient.NetworkInspect(
			ctx,
			hostCfg.NetworkMode.NetworkName(),
			types.NetworkInspectOptions{},
		)
		if client.IsErrNotFound(err) {
			_, err = dClient.NetworkCreate(
				ctx,
				hostCfg.NetworkMode.NetworkName(),
				types.NetworkCreate{
					Driver: "bridge",
					Options: map[string]string{
						"com.docker.network.bridge.enable_ip_masquerade": "true",
						"com.docker.network.driver.mtu":                  "1500",
					},
				},
			)
		}
		if err != nil {
			return types.ContainerJSON{}, fmt.Errorf("failed to create Docker network: %w", err)
		}
	}

	out, err := dClient.ImagePull(ctx, containerCfg.Image, types.ImagePullOptions{})
	defer func() { _ = out.Close() }()
	if err != nil {
		_, _ = io.Copy(os.Stderr, out)
		return types.ContainerJSON{}, fmt.Errorf("failed to pull Docker image: %w", err)
	}
	_, _ = io.Copy(io.Discard, out)

	created, err := dClient.ContainerCreate(ctx, containerCfg, hostCfg, nil, nil, containerName)
	if err != nil {
		return types.ContainerJSON{}, fmt.Errorf("failed to create container: %w", err)
	}
	containerID := created.ID

	if err := dClient.ContainerStart(ctx, containerID, types.ContainerStartOptions{}); err != nil {
		//nolint:contextcheck // Best effort background deletion.
		if deleteErr := ForceDeleteContainer(context.Background(), containerID); deleteErr != nil {
			err = multierr.Combine(err, deleteErr)
		}
		return types.ContainerJSON{}, fmt.Errorf("failed to start Docker container: %w", err)
	}

	containerInspect, err := dClient.ContainerInspect(ctx, containerID)
	if err != nil {
		//nolint:contextcheck // Best effort background deletion.
		if deleteErr := ForceDeleteContainer(context.Background(), containerID); deleteErr != nil {
			err = multierr.Combine(err, deleteErr)
		}
		return types.ContainerJSON{}, fmt.Errorf(
			"failed to inspect started Docker container: %w",
			err,
		)
	}

	return containerInspect, nil
}

func ForceDeleteContainer(ctx context.Context, containerID string) error {
	dClient, err := ClientFromEnv()
	if err != nil {
		return err
	}
	err = dClient.ContainerRemove(
		ctx,
		containerID,
		types.ContainerRemoveOptions{
			RemoveVolumes: true,
			Force:         true,
		},
	)
	if err != nil {
		return fmt.Errorf("failed to delete container: %w", err)
	}
	return nil
}

func PushImageToDifferentRegistry( //nolint:revive // Lots of args is fine in these tests.
	ctx context.Context,
	srcImage, destRegistry string,
	pullUsername, pullPassword string,
) error {
	dClient, err := ClientFromEnv()
	if err != nil {
		return err
	}

	_, _, err = dClient.ImageInspectWithRaw(
		ctx,
		srcImage,
	)
	if err != nil {
		if !client.IsErrNotFound(err) {
			return fmt.Errorf("failed to check if image is already present locally: %w", err)
		}

		out, err := dClient.ImagePull(
			ctx,
			srcImage,
			types.ImagePullOptions{RegistryAuth: authString(pullUsername, pullPassword)},
		)
		defer func() {
			if out != nil {
				_ = out.Close()
			}
		}()
		if err != nil {
			if out != nil {
				_, _ = io.Copy(os.Stderr, out)
			}
			return fmt.Errorf(
				"failed to pull image %q: %w",
				srcImage,
				err,
			)
		}
		_, _ = io.Copy(io.Discard, out)
	}

	srcRef, err := reference.ParseNormalizedNamed(srcImage)
	if err != nil {
		return fmt.Errorf("failed to parse source image name: %w", err)
	}
	srcTagged := srcRef.(reference.Tagged)

	destImage := fmt.Sprintf("%s/%s:%s", destRegistry, reference.Path(srcRef), srcTagged.Tag())

	if err := dClient.ImageTag(ctx, srcImage, destImage); err != nil {
		return fmt.Errorf("failed to retag image: %w", err)
	}
	defer func() { _, _ = dClient.ImageRemove(ctx, destImage, types.ImageRemoveOptions{}) }()

	out, err := dClient.ImagePush(
		ctx,
		destImage,
		types.ImagePushOptions{RegistryAuth: authString("", "")},
	)
	defer func() {
		if out != nil {
			_ = out.Close()
		}
	}()
	if err != nil {
		if out != nil {
			_, _ = io.Copy(os.Stderr, out)
		}
		return fmt.Errorf("failed to push retagged image: %w", err)
	}
	_, _ = io.Copy(io.Discard, out)

	return nil
}

func authString(username, password string) string {
	authConfig := types.AuthConfig{
		Username: username,
		Password: password,
	}
	encodedJSON, _ := json.Marshal(authConfig)

	return base64.URLEncoding.EncodeToString(encodedJSON)
}

func ReadFileFromContainer(ctx context.Context, containerID, fPath string) (string, error) {
	dClient, err := ClientFromEnv()
	if err != nil {
		return "", err
	}

	r, _, err := dClient.CopyFromContainer(ctx, containerID, fPath)
	if err != nil {
		return "", fmt.Errorf("failed to copy file from container: %w", err)
	}
	defer r.Close()
	tr := tar.NewReader(r)

	_, err = tr.Next()
	if err == io.EOF {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("failed to copy file from container: %w", err)
	}

	var b bytes.Buffer
	_, err = io.Copy(&b, tr) //nolint:gosec // Do not worry about DoS in e2e test.
	if err != nil {
		return "", fmt.Errorf("failed to copy file from container: %w", err)
	}

	return b.String(), nil
}
