package main

import (
	"context"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/lauralesteves/notas.lauraesteves.com/backend/internal/config"
	"github.com/lauralesteves/notas.lauraesteves.com/backend/internal/external/ses"
	"github.com/lauralesteves/notas.lauraesteves.com/backend/internal/repositories"
	"github.com/lauralesteves/notas.lauraesteves.com/backend/internal/services/report"
)

func main() {
	ctx := context.Background()
	cfg, err := config.LoadAWS(ctx)
	if err != nil {
		panic(err)
	}
	notes := repositories.NewDynamoNotesRepository(dynamodb.NewFromConfig(cfg), config.NotesTable())
	service := report.NewService(notes, ses.NewEmailSender(cfg), report.Config{
		From:    config.MailFrom(),
		To:      config.ReportTo(),
		SiteURL: config.SiteURL,
		Host:    config.FunctionName(),
	}, nil)

	lambda.Start(func(ctx context.Context) error { return service.Send(ctx) })
}
