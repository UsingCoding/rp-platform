package main

import (
	"context"
	"errors"
	"log/slog"

	"github.com/urfave/cli/v3"
	sdkworker "go.temporal.io/sdk/worker"
	"golang.org/x/sync/errgroup"

	"github.com/distributed-programming-2026/lib/runtime/http"

	"ordercreator/internal/ordercreator/config"
	"ordercreator/internal/ordercreator/infra/orderflow"
)

func (a *application) workerCmd() *cli.Command {
	return &cli.Command{
		Name: "worker",
		Action: func(ctx context.Context, _ *cli.Command) error {
			return worker(ctx, a.logger, a.conf)
		},
	}
}

func worker(ctx context.Context, logger *slog.Logger, conf config.Env) (err error) {
	temporalClient, err := dialTemporal(ctx, logger, conf)
	if err != nil {
		return err
	}
	defer temporalClient.Close()

	activities, closeClients, err := newActivities(conf)
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, closeClients())
	}()

	w := sdkworker.New(temporalClient, conf.Temporal.TaskQueue, sdkworker.Options{})
	orderflow.Register(w, activities)

	server, err := http.NewServer(http.Config{
		Address:      conf.HTTP.Address,
		GraceTimeout: conf.App.GraceTimeout,
	})
	if err != nil {
		return err
	}

	errGroup, egCtx := errgroup.WithContext(ctx)
	errGroup.Go(func() error {
		interrupt := make(chan any)
		go func() {
			<-egCtx.Done()
			close(interrupt)
		}()
		return w.Run(interrupt)
	})
	errGroup.Go(func() error {
		return server.Serve(egCtx)
	})
	return errGroup.Wait()
}
