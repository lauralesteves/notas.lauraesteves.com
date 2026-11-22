package repositories

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/lauralesteves/notas.lauraesteves.com/backend/internal/models"
)

type fakeDynamo struct {
	DynamoAPI
	put    *dynamodb.PutItemInput
	putErr error
}

func (f *fakeDynamo) PutItem(_ context.Context, in *dynamodb.PutItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error) {
	f.put = in
	return &dynamodb.PutItemOutput{}, f.putErr
}

func TestDynamoPutCondition(t *testing.T) {
	now := time.Date(2026, 11, 22, 18, 42, 0, 0, time.UTC)
	note := models.Note{Path: "a/b", Parent: "a", Body: "oi", Version: 3, CreatedAt: now, UpdatedAt: now, ExpiresAt: now.Add(models.NoteTTL)}
	f := &fakeDynamo{}
	repo := NewDynamoNotesRepository(f, "notas-notes")

	if err := repo.Put(context.Background(), note, WriteCondition{BaseVersion: 2, Now: now}); err != nil {
		t.Fatal(err)
	}
	if *f.put.ConditionExpression != "attribute_not_exists(#p) OR #e < :now OR #v = :base" {
		t.Errorf("condition = %s", *f.put.ConditionExpression)
	}
	if v := f.put.ExpressionAttributeValues[":base"].(*types.AttributeValueMemberN).Value; v != "2" {
		t.Errorf(":base = %s", v)
	}
	item := f.put.Item
	if item["kind"].(*types.AttributeValueMemberS).Value != "NOTE" ||
		item["updatedAt"].(*types.AttributeValueMemberS).Value != "2026-11-22T18:42:00Z" ||
		item["expiresAt"].(*types.AttributeValueMemberN).Value != strconv.FormatInt(note.ExpiresAt.Unix(), 10) ||
		item["size"].(*types.AttributeValueMemberN).Value != "2" {
		t.Errorf("item = %+v", item)
	}

	if err := repo.Put(context.Background(), note, WriteCondition{Force: true}); err != nil || f.put.ConditionExpression != nil {
		t.Errorf("force should be unconditional: %v", f.put.ConditionExpression)
	}

	f.putErr = &types.ConditionalCheckFailedException{}
	if err := repo.Put(context.Background(), note, WriteCondition{Now: now}); !errors.Is(err, ErrConditionFailed) {
		t.Errorf("got %v", err)
	}
}
