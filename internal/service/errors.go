package service

import (
	"context"
	"deployctl/internal/docker"
	"deployctl/internal/store"
	"errors"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type NotFoundError struct {
	Resource string
	Name     string
}

func (e *NotFoundError) Error() string {
	if e.Name == "" {
		return fmt.Sprintf("%s not found", e.Resource)
	}
	return fmt.Sprintf("%s %q not found", e.Resource, e.Name)
}

type InvalidArgumentError struct {
	Message string
}

func (e *InvalidArgumentError) Error() string {
	return e.Message
}

type ConflictError struct {
	Resource string
	Name     string
}

func (e *ConflictError) Error() string {
	if e.Name == "" {
		return fmt.Sprintf("%s already exists", e.Resource)
	}
	return fmt.Sprintf("%s %q already exists", e.Resource, e.Name)
}

func invalidArgument(message string) error {
	return &InvalidArgumentError{Message: message}
}

func deploymentNotFound(name string) error {
	return &NotFoundError{Resource: "deployment", Name: name}
}

func jobNotFound(id string) error {
	return &NotFoundError{Resource: "job", Name: id}
}

func deploymentConflict(name string) error {
	return &ConflictError{Resource: "deployment", Name: name}
}

func IsNotFound(err error) bool {
	var notFound *NotFoundError
	return errors.As(err, &notFound) || store.IsNotFound(err)
}

func repositoryError(name string, err error) error {
	if err == nil {
		return nil
	}
	if store.IsNotFound(err) {
		return deploymentNotFound(name)
	}
	return err
}

func repositoryInsertError(name string, err error) error {
	if err == nil {
		return nil
	}
	if store.IsConflict(err) {
		return deploymentConflict(name)
	}
	return repositoryError(name, err)
}

func jobError(id string, err error) error {
	if err == nil {
		return nil
	}
	if store.IsNotFound(err) {
		return jobNotFound(id)
	}
	return err
}

func userErrorMessage(err error) string {
	if message, ok := docker.UnavailableMessage(err); ok {
		return message
	}
	return err.Error()
}

func normalizeRPCError(err error) error {
	if err == nil {
		return nil
	}
	if message, ok := docker.UnavailableMessage(err); ok {
		return status.Error(codes.Unavailable, message)
	}
	var notFound *NotFoundError
	if errors.As(err, &notFound) {
		return status.Error(codes.NotFound, notFound.Error())
	}
	var conflict *ConflictError
	if errors.As(err, &conflict) {
		return status.Error(codes.AlreadyExists, conflict.Error())
	}
	var invalid *InvalidArgumentError
	if errors.As(err, &invalid) {
		return status.Error(codes.InvalidArgument, invalid.Error())
	}
	if errors.Is(err, context.Canceled) {
		return status.Error(codes.Canceled, err.Error())
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return status.Error(codes.DeadlineExceeded, err.Error())
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
