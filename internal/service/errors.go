package service

import (
	"context"
	"database/sql"
	"deployctl/internal/docker"
	"errors"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func IsNotFound(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}

func normalizeRPCError(err error) error {
	if err == nil {
		return nil
	}
	if message, ok := docker.UnavailableMessage(err); ok {
		return status.Error(codes.Unavailable, message)
	}
	return err
}

func normalizeUnaryError(
	ctx context.Context,
	req any,
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (any, error) {
	response, err := handler(ctx, req)
	return response, normalizeRPCError(err)
}
