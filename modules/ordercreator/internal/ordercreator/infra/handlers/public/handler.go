package public

import (
	"context"

	publicapi "ordercreator/api/server/publicapi"
	"ordercreator/internal/ordercreator/app"
)

func NewHandler(svc *app.Service) publicapi.StrictServerInterface {
	return &handler{
		svc: svc,
	}
}

type handler struct {
	svc *app.Service
}

func (h handler) Echo(ctx context.Context, req publicapi.EchoRequestObject) (publicapi.EchoResponseObject, error) {
	id, err := h.svc.Echo(ctx, req.Body.Body)
	if err != nil {
		return nil, err
	}

	return publicapi.Echo200JSONResponse{
		OperationID: id.String(),
	}, nil
}

func (h handler) CreateOrder(ctx context.Context, req publicapi.CreateOrderRequestObject) (publicapi.CreateOrderResponseObject, error) {
	order, workflowID, err := h.svc.CreateOrder(ctx, req.Body.Amount, req.Body.Address)
	if err != nil {
		return nil, err
	}

	return publicapi.CreateOrder202JSONResponse{
		OrderID:    order.ID.String(),
		WorkflowID: workflowID,
	}, nil
}
