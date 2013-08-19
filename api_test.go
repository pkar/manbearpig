package manbearpig

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewApiServer(t *testing.T) {
	_, err := NewAPIServer("9999", nil)
	if err != nil {
		t.Fatal(err)
	}
}

func TestJobHandler(t *testing.T) {
	sm, err := NewServiceManager()
	if err != nil {
		t.Fatal("Couldn't create service manager", err)
	}

	ap, _ := NewAPIServer("9999", sm)
	t.Logf("%+v", ap)

	b := strings.NewReader(`{"jobs": [{"user_id": 1, "product_id": 2, "payload": {}}], "auth": "abcd"}`)
	req, err := http.NewRequest("POST", "http://localhost:9999/job", b)
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	ap.JobsHandler(w, req)
	if w.Code != 200 {
		t.Log(w.Code, w.Body.String())
	}
}
