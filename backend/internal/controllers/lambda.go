package controllers

import (
	"context"
	"encoding/base64"
	"strings"

	"github.com/aws/aws-lambda-go/events"
)

// LambdaHTTP adapts a Handler to an HTTP API (payload v2) Lambda handler.
func LambdaHTTP(h Handler) func(context.Context, events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	return func(ctx context.Context, ev events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
		body := ev.Body
		if ev.IsBase64Encoded {
			decoded, err := base64.StdEncoding.DecodeString(ev.Body)
			if err != nil {
				return events.APIGatewayV2HTTPResponse{StatusCode: 400, Headers: map[string]string{"Content-Type": contentTypeText}, Body: "invalid body\n"}, nil
			}
			body = string(decoded)
		}
		headers := make(map[string]string, len(ev.Headers))
		for k, v := range ev.Headers {
			headers[strings.ToLower(k)] = v
		}
		resp := h(ctx, Request{
			Method:  ev.RequestContext.HTTP.Method,
			Path:    ev.PathParameters["path"],
			Query:   ev.QueryStringParameters,
			Headers: headers,
			Body:    body,
		})
		return events.APIGatewayV2HTTPResponse{StatusCode: resp.Status, Headers: resp.Headers, Body: resp.Body}, nil
	}
}
