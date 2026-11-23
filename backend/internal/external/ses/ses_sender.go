package ses

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/aws/aws-sdk-go-v2/service/sesv2/types"
)

// InlineImage is referenced from the HTML body as cid:<ContentID>.
// ContentID should be addr-spec shaped (id@domain), per RFC 2392.
type InlineImage struct {
	ContentID   string
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

// BuildInput sends the message as raw MIME so inline images sit in a
// multipart/related part next to the HTML (see BuildMIME).
func BuildInput(email Email, now time.Time) (*sesv2.SendEmailInput, error) {
	raw, err := BuildMIME(email, now)
	if err != nil {
		return nil, fmt.Errorf("build mime: %w", err)
	}
	return &sesv2.SendEmailInput{
		FromEmailAddress: aws.String(email.From),
		Destination:      &types.Destination{ToAddresses: email.To},
		Content:          &types.EmailContent{Raw: &types.RawMessage{Data: raw}},
	}, nil
}

func (s *sesSender) Send(ctx context.Context, email Email) error {
	in, err := BuildInput(email, time.Now())
	if err != nil {
		return err
	}
	if _, err := s.client.SendEmail(ctx, in); err != nil {
		return fmt.Errorf("ses send email: %w", err)
	}
	return nil
}
