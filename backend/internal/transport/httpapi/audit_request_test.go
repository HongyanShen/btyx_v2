package httpapi

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAuditBodySkipsFileAndPreservesHandlerInput(t *testing.T) {
	content := []byte("standalone file contents")
	for _, mediaType := range []string{"image/png", "application/octet-stream", "text/plain"} {
		req := httptest.NewRequest(http.MethodPost, "/upload", bytes.NewReader(content))
		req.Header.Set("Content-Type", mediaType)
		if mediaType == "text/plain" {
			req.Header.Set("Content-Disposition", "attachment; filename=note.txt")
		}

		var snapshot auditRequest
		if err := snapshot.readBody(req); err != nil {
			t.Fatal(err)
		}

		next, err := io.ReadAll(req.Body)
		if err != nil || !bytes.Equal(next, content) || snapshot.Body != "" || len(snapshot.BodyBase64) != 0 {
			t.Fatalf("unexpected capture or input change for %s", mediaType)
		}
	}
}

func TestAuditBodyPreservesNonUTF8Text(t *testing.T) {
	data := []byte{0xff, 0x41}
	req := httptest.NewRequest(http.MethodPost, "/text", bytes.NewReader(data))
	req.Header.Set("Content-Type", "text/plain; charset=iso-8859-1")
	var snapshot auditRequest
	if err := snapshot.readBody(req); err != nil {
		t.Fatal(err)
	}

	next, err := io.ReadAll(req.Body)
	if err != nil || !bytes.Equal(next, data) || !bytes.Equal(snapshot.BodyBase64, data) {
		t.Fatal("non-UTF8 text was altered")
	}
}
