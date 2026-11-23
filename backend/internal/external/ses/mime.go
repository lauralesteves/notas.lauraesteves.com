package ses

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"net/textproto"
	"strings"
	"time"
)

// BuildMIME renders the email as a raw RFC 5322 message:
//
//	multipart/alternative
//	├── text/plain
//	└── multipart/related   (only when there are inline images)
//	    ├── text/html
//	    └── image/* (Content-ID: <id>, inline, no filename)
//
// Inline images must live in a multipart/related next to the HTML, otherwise
// clients like Gmail do not resolve cid: references and show a broken image.
func BuildMIME(email Email, now time.Time) ([]byte, error) {
	var buf bytes.Buffer

	from, err := encodeAddress(email.From)
	if err != nil {
		return nil, fmt.Errorf("from: %w", err)
	}
	writeHeader(&buf, "From", from)
	writeHeader(&buf, "To", strings.Join(email.To, ", "))
	if len(email.ReplyTo) > 0 {
		writeHeader(&buf, "Reply-To", strings.Join(email.ReplyTo, ", "))
	}
	writeHeader(&buf, "Subject", mime.QEncoding.Encode("utf-8", email.Subject))
	writeHeader(&buf, "Date", now.Format(time.RFC1123Z))
	writeHeader(&buf, "MIME-Version", "1.0")

	alt := multipart.NewWriter(&buf)
	writeHeader(&buf, "Content-Type", `multipart/alternative; boundary="`+alt.Boundary()+`"`)
	buf.WriteString("\r\n")

	if err := writeQP(alt, "text/plain", email.Text); err != nil {
		return nil, err
	}

	if len(email.Inline) == 0 {
		if err := writeQP(alt, "text/html", email.HTML); err != nil {
			return nil, err
		}
		return buf.Bytes(), alt.Close()
	}

	var relBuf bytes.Buffer
	rel := multipart.NewWriter(&relBuf)
	if err := writeQP(rel, "text/html", email.HTML); err != nil {
		return nil, err
	}
	for _, img := range email.Inline {
		h := textproto.MIMEHeader{}
		h.Set("Content-Type", img.ContentType)
		h.Set("Content-Transfer-Encoding", "base64")
		h.Set("Content-ID", "<"+img.ContentID+">")
		// No filename: Gmail lists named parts as attachments even when the
		// HTML references them by cid.
		h.Set("Content-Disposition", "inline")
		w, err := rel.CreatePart(h)
		if err != nil {
			return nil, err
		}
		writeBase64(w, img.Data)
	}
	if err := rel.Close(); err != nil {
		return nil, err
	}

	h := textproto.MIMEHeader{}
	h.Set("Content-Type", `multipart/related; boundary="`+rel.Boundary()+`"`)
	w, err := alt.CreatePart(h)
	if err != nil {
		return nil, err
	}
	if _, err := w.Write(relBuf.Bytes()); err != nil {
		return nil, err
	}
	return buf.Bytes(), alt.Close()
}

func writeHeader(buf *bytes.Buffer, key, value string) {
	buf.WriteString(key + ": " + value + "\r\n")
}

// encodeAddress keeps a non-ASCII display name valid in the From header.
func encodeAddress(s string) (string, error) {
	addr, err := mail.ParseAddress(s)
	if err != nil {
		return "", err
	}
	return addr.String(), nil
}

func writeQP(mw *multipart.Writer, contentType, body string) error {
	h := textproto.MIMEHeader{}
	h.Set("Content-Type", contentType+`; charset="utf-8"`)
	h.Set("Content-Transfer-Encoding", "quoted-printable")
	w, err := mw.CreatePart(h)
	if err != nil {
		return err
	}
	qp := quotedprintable.NewWriter(w)
	if _, err := qp.Write([]byte(body)); err != nil {
		return err
	}
	return qp.Close()
}

func writeBase64(w interface{ Write([]byte) (int, error) }, data []byte) {
	enc := base64.StdEncoding.EncodeToString(data)
	for len(enc) > 76 {
		w.Write([]byte(enc[:76] + "\r\n"))
		enc = enc[76:]
	}
	w.Write([]byte(enc + "\r\n"))
}
