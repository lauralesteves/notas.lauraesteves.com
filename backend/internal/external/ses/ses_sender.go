package ses

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/aws/aws-sdk-go-v2/service/sesv2/types"
)

// InlineImage is referenced from the HTML body as cid:<ContentID>.
type InlineImage struct {
	ContentID   string
	FileName    string
	ContentType string
	Data        []byte
}

// Email is a ready-to-send message with both HTML and plain text bodies.
type Email struct {
	From    string
	To      []string
	ReplyTo []string
	Subject string
	HTML    string
	Text    string
	Inline  []InlineImage
}

type EmailSender interface {
	Send(ctx context.Context, email Email) error
}

type sesSender struct {
	client *sesv2.Client
}

func NewEmailSender(cfg aws.Config) EmailSender {
	return &sesSender{client: sesv2.NewFromConfig(cfg)}
}

func BuildInput(email Email) *sesv2.SendEmailInput {
	var attachments []types.Attachment
	for _, img := range email.Inline {
		attachments = append(attachments, types.Attachment{
			FileName:           aws.String(img.FileName),
			RawContent:         img.Data,
			ContentType:        aws.String(img.ContentType),
			ContentDisposition: types.AttachmentContentDispositionInline,
			ContentId:          aws.String(img.ContentID),
		})
	}
	return &sesv2.SendEmailInput{
		FromEmailAddress: aws.String(email.From),
		Destination:      &types.Destination{ToAddresses: email.To},
		ReplyToAddresses: email.ReplyTo,
		Content: &types.EmailContent{
			Simple: &types.Message{
				Subject: &types.Content{Data: aws.String(email.Subject), Charset: aws.String("UTF-8")},
				Body: &types.Body{
					Html: &types.Content{Data: aws.String(email.HTML), Charset: aws.String("UTF-8")},
					Text: &types.Content{Data: aws.String(email.Text), Charset: aws.String("UTF-8")},
				},
				Attachments: attachments,
			},
		},
	}
}

func (s *sesSender) Send(ctx context.Context, email Email) error {
	if _, err := s.client.SendEmail(ctx, BuildInput(email)); err != nil {
		return fmt.Errorf("ses send email: %w", err)
	}
	return nil
}
