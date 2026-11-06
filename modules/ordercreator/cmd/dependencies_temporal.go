package main

import (
	"context"
	"errors"
	"log/slog"

	"go.temporal.io/sdk/client"
	temporallog "go.temporal.io/sdk/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	deliveryinternalapi "delivery/api/server/internalapi"
	orderinternalapi "order/api/server/internalapi"
	paymentinternalapi "payment/api/server/internalapi"

	"ordercreator/internal/ordercreator/config"
	"ordercreator/internal/ordercreator/infra/orderflow"
)

func temporalOptions(logger *slog.Logger, conf config.Env) client.Options {
	return client.Options{
		HostPort:  conf.Temporal.Address,
		Namespace: conf.Temporal.Namespace,
		Logger:    temporallog.NewStructuredLogger(logger),
	}
}

// newLazyTemporalClient connects on first use, so the service starts while Temporal is unavailable.
func newLazyTemporalClient(logger *slog.Logger, conf config.Env) (client.Client, error) {
	return client.NewLazyClient(temporalOptions(logger, conf))
}

func dialTemporal(ctx context.Context, logger *slog.Logger, conf config.Env) (client.Client, error) {
	return client.DialContext(ctx, temporalOptions(logger, conf))
}

func newActivities(conf config.Env) (_ *orderflow.Activities, closeFunc func() error, err error) {
	var connections []*grpc.ClientConn
	closeFunc = func() error {
		errs := make([]error, 0, len(connections))
		for _, connection := range connections {
			errs = append(errs, connection.Close())
		}
		return errors.Join(errs...)
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, closeFunc())
		}
	}()

	dial := func(address string) (*grpc.ClientConn, error) {
		connection, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			return nil, err
		}
		connections = append(connections, connection)
		return connection, nil
	}

	orders, err := dial(conf.Services.OrderAddress)
	if err != nil {
		return nil, nil, err
	}
	payments, err := dial(conf.Services.PaymentAddress)
	if err != nil {
		return nil, nil, err
	}
	deliveries, err := dial(conf.Services.DeliveryAddress)
	if err != nil {
		return nil, nil, err
	}
	return orderflow.NewActivities(
		orderinternalapi.NewOrderServiceClient(orders),
		paymentinternalapi.NewPaymentServiceClient(payments),
		deliveryinternalapi.NewDeliveryServiceClient(deliveries),
	), closeFunc, nil
}
