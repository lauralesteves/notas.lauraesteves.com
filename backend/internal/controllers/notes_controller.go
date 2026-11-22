package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/lauralesteves/notas.lauraesteves.com/backend/internal/models"
	"github.com/lauralesteves/notas.lauraesteves.com/backend/internal/services"
)

type NotesController struct {
	service services.NotesService
}

func NewNotesController(service services.NotesService) *NotesController {
	return &NotesController{service: service}
}

// GetNote handles GET /notes/{path} and GET /notes/{path}?since={version}.
func (c *NotesController) GetNote(ctx context.Context, req Request) Response {
	var since *int64
	if raw, ok := req.Query["since"]; ok {
		v, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || v < 0 {
			return errorJSON(http.StatusBadRequest, "invalid_body", "since must be a non-negative integer")
		}
		since = &v
	}

	note, err := c.service.Get(ctx, req.Path)
	if err != nil {
		return serviceErrorJSON(req.Path, err)
	}
	dto := models.NewNoteDTO(req.Path, note)
	if since == nil {
		return jsonResponse(http.StatusOK, dto)
	}
	if dto.Version == *since {
		return jsonResponse(http.StatusOK, models.NoteUnchangedDTO{Changed: false, Version: dto.Version})
	}
	return jsonResponse(http.StatusOK, models.NotePollDTO{NoteDTO: dto, Changed: true})
}

// SaveNote handles PUT /notes/{path}.
func (c *NotesController) SaveNote(ctx context.Context, req Request) Response {
	var body models.SaveNoteRequest
	dec := json.NewDecoder(strings.NewReader(req.Body))
	if err := dec.Decode(&body); err != nil {
		return errorJSON(http.StatusBadRequest, "invalid_body", "malformed JSON body")
	}
	if body.Body == nil {
		return errorJSON(http.StatusBadRequest, "invalid_body", "body is required")
	}
	var base int64
	if body.BaseVersion != nil {
		base = *body.BaseVersion
	}

	result, err := c.service.Save(ctx, services.SaveInput{
		Path:         req.Path,
		Body:         *body.Body,
		BaseVersion:  base,
		Force:        body.Force,
		ConnectionID: req.Header("X-Connection-Id"),
	})
	if err != nil {
		return serviceErrorJSON(req.Path, err)
	}
	dto := models.NewNoteDTO(result.Path, result.Note)
	return jsonResponse(http.StatusOK, models.SaveNoteResponse{Path: dto.Path, Version: dto.Version, UpdatedAt: dto.UpdatedAt, ExpiresAt: dto.ExpiresAt})
}

// GetChildren handles GET /children/{path}.
func (c *NotesController) GetChildren(ctx context.Context, req Request) Response {
	children, err := c.service.Children(ctx, req.Path)
	if err != nil {
		return serviceErrorJSON(req.Path, err)
	}
	out := models.ChildrenDTO{Path: req.Path, Children: make([]models.ChildDTO, 0, len(children))}
	for _, ch := range children {
		out.Children = append(out.Children, models.ChildDTO{Path: ch.Path, UpdatedAt: ch.UpdatedAt.UTC()})
	}
	return jsonResponse(http.StatusOK, out)
}

// GetRaw handles GET /raw/{path}.
func (c *NotesController) GetRaw(ctx context.Context, req Request) Response {
	note, err := c.service.Get(ctx, req.Path)
	if err != nil {
		return serviceErrorText(req.Path, err)
	}
	if note == nil {
		return textResponse(http.StatusNotFound, "not found\n")
	}
	body := note.Body
	if !strings.HasSuffix(body, "\n") {
		body += "\n"
	}
	return textResponse(http.StatusOK, body)
}

// SaveRaw handles PUT /raw/{path} as a last-write-wins save.
func (c *NotesController) SaveRaw(ctx context.Context, req Request) Response {
	result, err := c.service.Save(ctx, services.SaveInput{
		Path:         req.Path,
		Body:         req.Body,
		Force:        true,
		ConnectionID: req.Header("X-Connection-Id"),
	})
	if err != nil {
		return serviceErrorText(req.Path, err)
	}
	if result.Note == nil {
		return textResponse(http.StatusOK, "deleted\n")
	}
	return textResponse(http.StatusOK, fmt.Sprintf("saved v%d, expires %s\n", result.Note.Version, result.Note.ExpiresAt.UTC().Format(time.RFC3339)))
}
