package repositories

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/lauralesteves/notas.lauraesteves.com/backend/internal/models"
)

type connectionItem struct {
	ConnectionID string `dynamodbav:"connectionId"`
	Path         string `dynamodbav:"path"`
	ConnectedAt  string `dynamodbav:"connectedAt,omitempty"`
	TTL          int64  `dynamodbav:"ttl"`
}

func (i connectionItem) toModel() models.Connection {
	return models.Connection{
		ConnectionID: i.ConnectionID,
		Path:         i.Path,
		ConnectedAt:  parseTime(i.ConnectedAt),
		ExpiresAt:    time.Unix(i.TTL, 0).UTC(),
	}
}

type dynamoConnectionsRepository struct {
	db    DynamoAPI
	table string
}

func NewDynamoConnectionsRepository(db DynamoAPI, table string) ConnectionsRepository {
	return &dynamoConnectionsRepository{db: db, table: table}
}

func (r *dynamoConnectionsRepository) key(id string) map[string]types.AttributeValue {
	return map[string]types.AttributeValue{"connectionId": &types.AttributeValueMemberS{Value: id}}
}

func (r *dynamoConnectionsRepository) Put(ctx context.Context, conn models.Connection) error {
	item, err := attributevalue.MarshalMap(connectionItem{
		ConnectionID: conn.ConnectionID,
		Path:         conn.Path,
		ConnectedAt:  formatTime(conn.ConnectedAt),
		TTL:          conn.ExpiresAt.Unix(),
	})
	if err != nil {
		return fmt.Errorf("marshal connection: %w", err)
	}
	if _, err := r.db.PutItem(ctx, &dynamodb.PutItemInput{TableName: aws.String(r.table), Item: item}); err != nil {
		return fmt.Errorf("put connection: %w", err)
	}
	return nil
}

func (r *dynamoConnectionsRepository) Get(ctx context.Context, connectionID string) (*models.Connection, error) {
	out, err := r.db.GetItem(ctx, &dynamodb.GetItemInput{TableName: aws.String(r.table), Key: r.key(connectionID)})
	if err != nil {
		return nil, fmt.Errorf("get connection: %w", err)
	}
	if out.Item == nil {
		return nil, nil
	}
	var item connectionItem
	if err := attributevalue.UnmarshalMap(out.Item, &item); err != nil {
		return nil, fmt.Errorf("unmarshal connection: %w", err)
	}
	c := item.toModel()
	return &c, nil
}

func (r *dynamoConnectionsRepository) Delete(ctx context.Context, connectionID string) error {
	if _, err := r.db.DeleteItem(ctx, &dynamodb.DeleteItemInput{TableName: aws.String(r.table), Key: r.key(connectionID)}); err != nil {
		return fmt.Errorf("delete connection: %w", err)
	}
	return nil
}

func (r *dynamoConnectionsRepository) ListByPath(ctx context.Context, path string, now time.Time) ([]models.Connection, error) {
	out := []models.Connection{}
	p := dynamodb.NewQueryPaginator(r.db, &dynamodb.QueryInput{
		TableName:                aws.String(r.table),
		IndexName:                aws.String(IndexByPath),
		KeyConditionExpression:   aws.String("#p = :path"),
		FilterExpression:         aws.String("#t > :now"),
		ExpressionAttributeNames: map[string]string{"#p": "path", "#t": "ttl"},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":path": &types.AttributeValueMemberS{Value: path},
			":now":  epoch(now),
		},
	})
	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("query connections: %w", err)
		}
		var items []connectionItem
		if err := attributevalue.UnmarshalListOfMaps(page.Items, &items); err != nil {
			return nil, fmt.Errorf("unmarshal connections: %w", err)
		}
		for _, i := range items {
			out = append(out, i.toModel())
		}
	}
	return out, nil
}
