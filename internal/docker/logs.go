package docker

import (
	"context"
	"fmt"
	"strconv"
	"sync"

	"deployctl/internal/store"

	"github.com/docker/compose/v5/pkg/api"
)

type LogEntry struct {
	Container string
	Message   string
}

type LogOptions struct {
	Follow bool
	Lines  int32
}

type LogConsumer func(LogEntry) error

func ComposeLogs(ctx context.Context, repository *store.Repository, options LogOptions, consume LogConsumer) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	service, project, dockerCLI, err := loadProject(ctx, repository)
	if err != nil {
		return err
	}

	consumer := &composeLogConsumer{consume: consume, cancel: cancel}
	if err := service.Logs(ctx, project.Name, consumer, api.LogOptions{
		Project: project,
		Follow:  options.Follow,
		Tail:    strconv.FormatInt(int64(options.Lines), 10),
	}); err != nil {
		if consumerErr := consumer.err(); consumerErr != nil {
			return consumerErr
		}
		if unavailable := dockerUnavailableError(dockerCLI, err); unavailable != nil {
			return unavailable
		}
		return fmt.Errorf("read compose logs: %w", err)
	}

	return consumer.err()
}

type composeLogConsumer struct {
	consume LogConsumer
	cancel  context.CancelFunc
	mu      sync.Mutex
	sendErr error
}

func (c *composeLogConsumer) Log(containerName, message string) {
	c.send(LogEntry{Container: containerName, Message: message})
}

func (c *composeLogConsumer) Err(containerName, message string) {
	c.send(LogEntry{Container: containerName, Message: message})
}

func (c *composeLogConsumer) Status(string, string) {}

func (c *composeLogConsumer) send(entry LogEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.sendErr != nil {
		return
	}
	if err := c.consume(entry); err != nil {
		c.sendErr = err
		c.cancel()
	}
}

func (c *composeLogConsumer) err() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.sendErr
}
