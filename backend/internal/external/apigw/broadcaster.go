package apigw

import (
	"context"
	"errors"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/apigatewaymanagementapi"
	"github.com/aws/aws-sdk-go-v2/service/apigatewaymanagementapi/types"
	"github.com/lauralesteves/notas.lauraesteves.com/backend/internal/services"
)

type broadcaster struct {
	client *apigatewaymanagementapi.Client
}

// NewBroadcaster posts to WebSocket connections through the API Gateway management API.
func NewBroadcaster(cfg aws.Config, endpoint string) services.Broadcaster {
	client := apigatewaymanagementapi.NewFromConfig(cfg, func(o *apigatewaymanagementapi.Options) {
		o.BaseEndpoint = aws.String(endpoint)
	})
	return &broadcaster{client: client}
}

func (b *broadcaster) Post(ctx context.Context, connectionID string, payload []byte) error {
	_, err := b.client.PostToConnection(ctx, &apigatewaymanagementapi.PostToConnectionInput{
		ConnectionId: aws.String(connectionID),
		Data:         payload,
	})
	var gone *types.GoneException
	if errors.As(err, &gone) {
		return services.ErrGone
	}
	return err
}
