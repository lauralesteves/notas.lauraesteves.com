package models

import "time"

type NoteDTO struct {
	Path      string     `json:"path"`
	Body      string     `json:"body"`
	Version   int64      `json:"version"`
	UpdatedAt *time.Time `json:"updatedAt"`
	ExpiresAt *time.Time `json:"expiresAt"`
}

type NotePollDTO struct {
	NoteDTO
	Changed bool `json:"changed"`
}

type NoteUnchangedDTO struct {
	Changed bool  `json:"changed"`
	Version int64 `json:"version"`
}

type SaveNoteRequest struct {
	Body        *string `json:"body"`
	BaseVersion *int64  `json:"baseVersion"`
	Force       bool    `json:"force"`
}

type SaveNoteResponse struct {
	Path      string     `json:"path"`
	Version   int64      `json:"version"`
	UpdatedAt *time.Time `json:"updatedAt"`
	ExpiresAt *time.Time `json:"expiresAt"`
}

type ChildDTO struct {
	Path      string    `json:"path"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type ChildrenDTO struct {
	Path     string     `json:"path"`
	Children []ChildDTO `json:"children"`
}

type ErrorDTO struct {
	Error   string   `json:"error"`
	Message string   `json:"message"`
	Note    *NoteDTO `json:"note,omitempty"`
}

type UpdateMessage struct {
	Type      string     `json:"type"`
	Path      string     `json:"path"`
	Version   int64      `json:"version"`
	Body      string     `json:"body"`
	UpdatedAt *time.Time `json:"updatedAt"`
	ExpiresAt *time.Time `json:"expiresAt"`
}

type ChangedMessage struct {
	Type    string `json:"type"`
	Path    string `json:"path"`
	Version int64  `json:"version"`
}

type PresenceMessage struct {
	Type  string `json:"type"`
	Path  string `json:"path"`
	Count int    `json:"count"`
}

type PongMessage struct {
	Type         string `json:"type"`
	ConnectionID string `json:"connectionId"`
	Count        int    `json:"count"`
}

type ClientMessage struct {
	Action string `json:"action"`
}

func NewNoteDTO(path string, n *Note) NoteDTO {
	if n == nil {
		return NoteDTO{Path: path}
	}
	u, e := n.UpdatedAt.UTC(), n.ExpiresAt.UTC()
	return NoteDTO{Path: n.Path, Body: n.Body, Version: n.Version, UpdatedAt: &u, ExpiresAt: &e}
}
