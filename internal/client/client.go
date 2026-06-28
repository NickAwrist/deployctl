package client

import (
	"context"
	"fmt"
	"net"
	"time"

	"deployctl/internal"
	"deployctl/internal/rpc"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Client struct {
	connection *grpc.ClientConn
	Deployment rpc.DeploymentServiceClient
	Env        rpc.EnvServiceClient
	Job        rpc.JobServiceClient
	System     rpc.SystemServiceClient
}

func Dial(ctx context.Context, socketPath string) (*Client, error) {
	if socketPath == "" {
		socketPath = internal.GetSocketPath()
	}

	connection, err := grpc.DialContext(
		ctx,
		"passthrough:///unix",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			var dialer net.Dialer
			return dialer.DialContext(ctx, "unix", socketPath)
		}),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, fmt.Errorf("connect to deployctld at %s: %w", socketPath, err)
	}

	return &Client{
		connection: connection,
		Deployment: rpc.NewDeploymentServiceClient(connection),
		Env:        rpc.NewEnvServiceClient(connection),
		Job:        rpc.NewJobServiceClient(connection),
		System:     rpc.NewSystemServiceClient(connection),
	}, nil
}

func DialDefault(ctx context.Context) (*Client, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return Dial(ctx, "")
}

func (c *Client) Close() error {
	return c.connection.Close()
}
