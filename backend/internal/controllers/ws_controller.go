package controllers

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/aws/aws-lambda-go/events"
	"github.com/lauralesteves/notas.lauraesteves.com/backend/internal/services"
)

type WSController struct {
	service services.ConnectionsService
}

func NewWSController(service services.ConnectionsService) *WSController {
	return &WSController{service: service}
}

func wsResponse(status int) events.APIGatewayProxyResponse {
	return events.APIGatewayProxyResponse{StatusCode: status}
}

// Connect handles $connect; a non-2xx status refuses the connection.
func (c *WSController) Connect(ctx context.Context, ev events.APIGatewayWebsocketProxyRequest) (events.APIGatewayProxyResponse, error) {
	err := c.service.Connect(ctx, ev.RequestContext.ConnectionID, ev.QueryStringParameters["path"])
	switch {
	case err == nil:
		return wsResponse(http.StatusOK), nil
	case errors.Is(err, services.ErrInvalidPath):
		return wsResponse(http.StatusBadRequest), nil
	case errors.Is(err, services.ErrTooManyConnections):
		return wsResponse(http.StatusTooManyRequests), nil
	default:
		slog.Error("ws connect", "error", err)
		return wsResponse(http.StatusInternalServerError), nil
	}
}

func (c *WSController) Disconnect(ctx context.Context, ev events.APIGatewayWebsocketProxyRequest) (events.APIGatewayProxyResponse, error) {
	if err := c.service.Disconnect(ctx, ev.RequestContext.ConnectionID); err != nil {
		slog.Error("ws disconnect", "error", err)
		return wsResponse(http.StatusInternalServerError), nil
	}
	return wsResponse(http.StatusOK), nil
}

func (c *WSController) Default(ctx context.Context, ev events.APIGatewayWebsocketProxyRequest) (events.APIGatewayProxyResponse, error) {
	if err := c.service.HandleMessage(ctx, ev.RequestContext.ConnectionID, ev.Body); err != nil && !errors.Is(err, services.ErrGone) {
		slog.Error("ws message", "error", err)
		return wsResponse(http.StatusInternalServerError), nil
	}
	return wsResponse(http.StatusOK), nil
}
