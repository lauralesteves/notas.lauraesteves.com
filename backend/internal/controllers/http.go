package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/lauralesteves/notas.lauraesteves.com/backend/internal/models"
	"github.com/lauralesteves/notas.lauraesteves.com/backend/internal/services"
)

// Request is a transport-agnostic HTTP request, built from a Lambda event or a net/http request.
type Request struct {
	Method  string
	Path    string
	Query   map[string]string
	Headers map[string]string
	Body    string
}

func (r Request) Header(name string) string {
	if v, ok := r.Headers[strings.ToLower(name)]; ok {
		return v
	}
	for k, v := range r.Headers {
		if strings.EqualFold(k, name) {
			return v
		}
	}
	return ""
}

type Response struct {
	Status  int
	Headers map[string]string
	Body    string
}

type Handler func(ctx context.Context, req Request) Response

const (
	contentTypeJSON = "application/json; charset=utf-8"
	contentTypeText = "text/plain; charset=utf-8"
)

func jsonResponse(status int, v any) Response {
	b, err := json.Marshal(v)
	if err != nil {
		slog.Error("marshal response", "error", err)
		return Response{Status: http.StatusInternalServerError, Headers: map[string]string{"Content-Type": contentTypeJSON}, Body: `{"error":"internal","message":"marshal response"}`}
	}
	return Response{Status: status, Headers: map[string]string{"Content-Type": contentTypeJSON}, Body: string(b)}
}

func textResponse(status int, body string) Response {
	return Response{Status: status, Headers: map[string]string{"Content-Type": contentTypeText}, Body: body}
}

func errorJSON(status int, code, message string) Response {
	return jsonResponse(status, models.ErrorDTO{Error: code, Message: message})
}

func serviceErrorJSON(path string, err error) Response {
	var conflict *services.ConflictError
	switch {
	case errors.As(err, &conflict):
		note := models.NewNoteDTO(path, conflict.Current)
		return jsonResponse(http.StatusConflict, models.ErrorDTO{Error: "conflict", Message: conflict.Error(), Note: &note})
	case errors.Is(err, services.ErrInvalidPath):
		return errorJSON(http.StatusBadRequest, "invalid_path", err.Error())
	case errors.Is(err, services.ErrInvalidBody):
		return errorJSON(http.StatusBadRequest, "invalid_body", err.Error())
	case errors.Is(err, services.ErrTooLarge):
		return errorJSON(http.StatusRequestEntityTooLarge, "too_large", err.Error())
	default:
		slog.Error("request failed", "path", path, "error", err)
		return errorJSON(http.StatusInternalServerError, "internal", "internal error")
	}
}

func serviceErrorText(path string, err error) Response {
	switch {
	case errors.Is(err, services.ErrInvalidPath):
		return textResponse(http.StatusBadRequest, "invalid path\n")
	case errors.Is(err, services.ErrInvalidBody):
		return textResponse(http.StatusBadRequest, "invalid body\n")
	case errors.Is(err, services.ErrTooLarge):
		return textResponse(http.StatusRequestEntityTooLarge, "too large\n")
	default:
		slog.Error("raw request failed", "path", path, "error", err)
		return textResponse(http.StatusInternalServerError, "internal error\n")
	}
}
