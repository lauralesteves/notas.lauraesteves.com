package main

import (
	"context"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/lauralesteves/notas.lauraesteves.com/backend/internal/config"
	"github.com/lauralesteves/notas.lauraesteves.com/backend/internal/controllers"
	"github.com/lauralesteves/notas.lauraesteves.com/backend/internal/external/apigw"
	"github.com/lauralesteves/notas.lauraesteves.com/backend/internal/repositories"
	"github.com/lauralesteves/notas.lauraesteves.com/backend/internal/services"
)

func main() {
	ctx := context.Background()
	cfg, err := config.LoadAWS(ctx)
	if err != nil {
		panic(err)
	}
	conns := repositories.NewDynamoConnectionsRepository(dynamodb.NewFromConfig(cfg), config.ConnectionsTable())
	broadcast := services.NewBroadcastService(conns, apigw.NewBroadcaster(cfg, config.WSManagementEndpoint()), nil)
	controller := controllers.NewWSController(services.NewConnectionsService(conns, broadcast, nil))

	lambda.Start(controller.Connect)
}
