package repositories

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/lauralesteves/notas.lauraesteves.com/backend/internal/models"
)

const (
	IndexByParent  = "byParent"
	IndexByUpdated = "byUpdated"
	IndexByPath    = "byPath"
	timeLayout     = "2006-01-02T15:04:05Z"
)

// DynamoAPI is the subset of the DynamoDB client used by the repositories.
type DynamoAPI interface {
	GetItem(ctx context.Context, in *dynamodb.GetItemInput, opts ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error)
	PutItem(ctx context.Context, in *dynamodb.PutItemInput, opts ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error)
	DeleteItem(ctx context.Context, in *dynamodb.DeleteItemInput, opts ...func(*dynamodb.Options)) (*dynamodb.DeleteItemOutput, error)
	Query(ctx context.Context, in *dynamodb.QueryInput, opts ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error)
	Scan(ctx context.Context, in *dynamodb.ScanInput, opts ...func(*dynamodb.Options)) (*dynamodb.ScanOutput, error)
}

type noteItem struct {
	Path      string `dynamodbav:"path"`
	Parent    string `dynamodbav:"parent"`
	Body      string `dynamodbav:"body"`
	Version   int64  `dynamodbav:"version"`
	Size      int    `dynamodbav:"size"`
	CreatedAt string `dynamodbav:"createdAt"`
	UpdatedAt string `dynamodbav:"updatedAt"`
	Kind      string `dynamodbav:"kind"`
	ExpiresAt int64  `dynamodbav:"expiresAt"`
}

func formatTime(t time.Time) string { return t.UTC().Format(timeLayout) }

func parseTime(s string) time.Time {
	t, _ := time.Parse(timeLayout, s)
	return t
}

func toNoteItem(n models.Note) noteItem {
	return noteItem{
		Path:      n.Path,
		Parent:    n.Parent,
		Body:      n.Body,
		Version:   n.Version,
		Size:      len(n.Body),
		CreatedAt: formatTime(n.CreatedAt),
		UpdatedAt: formatTime(n.UpdatedAt),
		Kind:      models.NoteKind,
		ExpiresAt: n.ExpiresAt.Unix(),
	}
}

func (i noteItem) toModel() models.Note {
	return models.Note{
		Path:      i.Path,
		Parent:    i.Parent,
		Body:      i.Body,
		Version:   i.Version,
		CreatedAt: parseTime(i.CreatedAt),
		UpdatedAt: parseTime(i.UpdatedAt),
		ExpiresAt: time.Unix(i.ExpiresAt, 0).UTC(),
	}
}

type dynamoNotesRepository struct {
	db    DynamoAPI
	table string
}

func NewDynamoNotesRepository(db DynamoAPI, table string) NotesRepository {
	return &dynamoNotesRepository{db: db, table: table}
}

func epoch(t time.Time) types.AttributeValue {
	return &types.AttributeValueMemberN{Value: strconv.FormatInt(t.Unix(), 10)}
}

func (r *dynamoNotesRepository) key(path string) map[string]types.AttributeValue {
	return map[string]types.AttributeValue{"path": &types.AttributeValueMemberS{Value: path}}
}

func writeCondition(cond WriteCondition) (*string, map[string]string, map[string]types.AttributeValue) {
	if cond.Force {
		return nil, nil, nil
	}
	return aws.String("attribute_not_exists(#p) OR #e < :now OR #v = :base"),
		map[string]string{"#p": "path", "#e": "expiresAt", "#v": "version"},
		map[string]types.AttributeValue{
			":now":  epoch(cond.Now),
			":base": &types.AttributeValueMemberN{Value: strconv.FormatInt(cond.BaseVersion, 10)},
		}
}

func mapConditionError(err error) error {
	var ccf *types.ConditionalCheckFailedException
	if errors.As(err, &ccf) {
		return ErrConditionFailed
	}
	return err
}

func (r *dynamoNotesRepository) Get(ctx context.Context, path string) (*models.Note, error) {
	out, err := r.db.GetItem(ctx, &dynamodb.GetItemInput{
		TableName:      aws.String(r.table),
		Key:            r.key(path),
		ConsistentRead: aws.Bool(true),
	})
	if err != nil {
		return nil, fmt.Errorf("get note: %w", err)
	}
	if out.Item == nil {
		return nil, nil
	}
	var item noteItem
	if err := attributevalue.UnmarshalMap(out.Item, &item); err != nil {
		return nil, fmt.Errorf("unmarshal note: %w", err)
	}
	n := item.toModel()
	return &n, nil
}

func (r *dynamoNotesRepository) Put(ctx context.Context, note models.Note, cond WriteCondition) error {
	item, err := attributevalue.MarshalMap(toNoteItem(note))
	if err != nil {
		return fmt.Errorf("marshal note: %w", err)
	}
	expr, names, values := writeCondition(cond)
	_, err = r.db.PutItem(ctx, &dynamodb.PutItemInput{
		TableName:                 aws.String(r.table),
		Item:                      item,
		ConditionExpression:       expr,
		ExpressionAttributeNames:  names,
		ExpressionAttributeValues: values,
	})
	if err != nil {
		return mapConditionError(err)
	}
	return nil
}

func (r *dynamoNotesRepository) Delete(ctx context.Context, path string, cond WriteCondition) error {
	expr, names, values := writeCondition(cond)
	_, err := r.db.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName:                 aws.String(r.table),
		Key:                       r.key(path),
		ConditionExpression:       expr,
		ExpressionAttributeNames:  names,
		ExpressionAttributeValues: values,
	})
	if err != nil {
		return mapConditionError(err)
	}
	return nil
}

type indexItem struct {
	Path      string `dynamodbav:"path"`
	UpdatedAt string `dynamodbav:"updatedAt"`
}

func (r *dynamoNotesRepository) ListChildren(ctx context.Context, parent string, now time.Time) ([]models.ChildNote, error) {
	out := []models.ChildNote{}
	input := &dynamodb.QueryInput{
		TableName:                aws.String(r.table),
		IndexName:                aws.String(IndexByParent),
		KeyConditionExpression:   aws.String("#parent = :parent"),
		FilterExpression:         aws.String("#e > :now"),
		ExpressionAttributeNames: map[string]string{"#parent": "parent", "#e": "expiresAt"},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":parent": &types.AttributeValueMemberS{Value: parent},
			":now":    epoch(now),
		},
	}
	p := dynamodb.NewQueryPaginator(r.db, input)
	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("query children: %w", err)
		}
		var items []indexItem
		if err := attributevalue.UnmarshalListOfMaps(page.Items, &items); err != nil {
			return nil, fmt.Errorf("unmarshal children: %w", err)
		}
		for _, i := range items {
			out = append(out, models.ChildNote{Path: i.Path, UpdatedAt: parseTime(i.UpdatedAt)})
		}
	}
	return out, nil
}

func (r *dynamoNotesRepository) CountActive(ctx context.Context, now time.Time) (int, error) {
	total := 0
	p := dynamodb.NewScanPaginator(r.db, &dynamodb.ScanInput{
		TableName:                 aws.String(r.table),
		Select:                    types.SelectCount,
		FilterExpression:          aws.String("#e > :now"),
		ExpressionAttributeNames:  map[string]string{"#e": "expiresAt"},
		ExpressionAttributeValues: map[string]types.AttributeValue{":now": epoch(now)},
	})
	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			return 0, fmt.Errorf("scan count: %w", err)
		}
		total += int(page.Count)
	}
	return total, nil
}

func (r *dynamoNotesRepository) CountUpdatedSince(ctx context.Context, since time.Time) (int, error) {
	total := 0
	p := dynamodb.NewQueryPaginator(r.db, &dynamodb.QueryInput{
		TableName:                aws.String(r.table),
		IndexName:                aws.String(IndexByUpdated),
		Select:                   types.SelectCount,
		KeyConditionExpression:   aws.String("#k = :kind AND #u >= :since"),
		ExpressionAttributeNames: map[string]string{"#k": "kind", "#u": "updatedAt"},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":kind":  &types.AttributeValueMemberS{Value: models.NoteKind},
			":since": &types.AttributeValueMemberS{Value: formatTime(since)},
		},
	})
	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			return 0, fmt.Errorf("query updated count: %w", err)
		}
		total += int(page.Count)
	}
	return total, nil
}

func (r *dynamoNotesRepository) ListRecent(ctx context.Context, limit int, now time.Time) ([]models.RecentNote, error) {
	out := []models.RecentNote{}
	p := dynamodb.NewQueryPaginator(r.db, &dynamodb.QueryInput{
		TableName:                aws.String(r.table),
		IndexName:                aws.String(IndexByUpdated),
		KeyConditionExpression:   aws.String("#k = :kind"),
		FilterExpression:         aws.String("#e > :now"),
		ScanIndexForward:         aws.Bool(false),
		Limit:                    aws.Int32(int32(limit)),
		ExpressionAttributeNames: map[string]string{"#k": "kind", "#e": "expiresAt"},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":kind": &types.AttributeValueMemberS{Value: models.NoteKind},
			":now":  epoch(now),
		},
	})
	for p.HasMorePages() && len(out) < limit {
		page, err := p.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("query recent: %w", err)
		}
		var items []indexItem
		if err := attributevalue.UnmarshalListOfMaps(page.Items, &items); err != nil {
			return nil, fmt.Errorf("unmarshal recent: %w", err)
		}
		for _, i := range items {
			if len(out) == limit {
				break
			}
			out = append(out, models.RecentNote{Path: i.Path, UpdatedAt: parseTime(i.UpdatedAt)})
		}
	}
	return out, nil
}
