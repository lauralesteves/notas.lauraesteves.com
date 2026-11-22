package config

import (
	"context"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
)

const (
	SiteURL                 = "https://notas.lauraesteves.com"
	defaultNotesTable       = "notas-notes"
	defaultConnectionsTable = "notas-connections"
)

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func NotesTable() string { return getEnv("NOTES_TABLE", defaultNotesTable) }

func ConnectionsTable() string { return getEnv("CONNECTIONS_TABLE", defaultConnectionsTable) }

// WSManagementEndpoint is the https endpoint of the WebSocket API stage used for @connections.
func WSManagementEndpoint() string { return os.Getenv("WS_MANAGEMENT_ENDPOINT") }

func MailFrom() string { return os.Getenv("MAIL_FROM") }

func ReportTo() string { return os.Getenv("REPORT_TO") }

func FunctionName() string { return getEnv("AWS_LAMBDA_FUNCTION_NAME", "notas-weekly-report") }

func LoadAWS(ctx context.Context) (aws.Config, error) {
	cfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		return aws.Config{}, fmt.Errorf("load aws config: %w", err)
	}
	return cfg, nil
}
