package httpapi_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rrjLoki/TrainTrainSkill/internal/httpapi"
	"github.com/rrjLoki/TrainTrainSkill/internal/store"
)

func newHandler() http.Handler {
	return httpapi.New(httpapi.Deps{Store: store.New()})
}

func do(t *testing.T, h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var r *http.Request
	if body == "" {
		r = httptest.NewRequest(method, path, nil)
	} else {
		r = httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestHealthz(t *testing.T) {
	w := do(t, newHandler(), http.MethodGet, "/healthz", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got["status"] != "ok" {
		t.Fatalf("status field = %v, want ok", got["status"])
	}
}

func TestItemLifecycle(t *testing.T) {
	h := newHandler()

	// create
	w := do(t, h, http.MethodPost, "/api/items", `{"title":"写测试"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want 201, body = %s", w.Code, w.Body)
	}
	var created store.Item
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created: %v", err)
	}
	if created.ID == "" || created.Title != "写测试" || created.Done {
		t.Fatalf("unexpected item: %+v", created)
	}

	// list
	w = do(t, h, http.MethodGet, "/api/items", "")
	if w.Code != http.StatusOK {
		t.Fatalf("list status = %d", w.Code)
	}
	var list struct {
		Items []store.Item `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(list.Items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(list.Items))
	}

	// patch
	w = do(t, h, http.MethodPatch, "/api/items/"+created.ID, `{"done":true}`)
	if w.Code != http.StatusOK {
		t.Fatalf("patch status = %d, body = %s", w.Code, w.Body)
	}
	var updated store.Item
	if err := json.Unmarshal(w.Body.Bytes(), &updated); err != nil {
		t.Fatalf("decode updated: %v", err)
	}
	if !updated.Done || updated.Title != "写测试" {
		t.Fatalf("unexpected updated item: %+v", updated)
	}

	// get
	w = do(t, h, http.MethodGet, "/api/items/"+created.ID, "")
	if w.Code != http.StatusOK {
		t.Fatalf("get status = %d", w.Code)
	}

	// delete
	w = do(t, h, http.MethodDelete, "/api/items/"+created.ID, "")
	if w.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, want 204", w.Code)
	}

	// get after delete
	w = do(t, h, http.MethodGet, "/api/items/"+created.ID, "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("get after delete status = %d, want 404", w.Code)
	}
}

func TestItemValidation(t *testing.T) {
	h := newHandler()

	cases := []struct {
		name   string
		method string
		path   string
		body   string
		want   int
	}{
		{"empty title", http.MethodPost, "/api/items", `{"title":"   "}`, http.StatusBadRequest},
		{"missing title", http.MethodPost, "/api/items", `{}`, http.StatusBadRequest},
		{"unknown field", http.MethodPost, "/api/items", `{"title":"x","oops":1}`, http.StatusBadRequest},
		{"bad json", http.MethodPost, "/api/items", `{`, http.StatusBadRequest},
		{"patch nothing", http.MethodPatch, "/api/items/abc", `{}`, http.StatusBadRequest},
		{"patch missing", http.MethodPatch, "/api/items/nope", `{"done":true}`, http.StatusNotFound},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := do(t, h, tc.method, tc.path, tc.body)
			if w.Code != tc.want {
				t.Fatalf("status = %d, want %d, body = %s", w.Code, tc.want, w.Body)
			}
		})
	}
}

func TestChatWithoutLLM(t *testing.T) {
	w := do(t, newHandler(), http.MethodPost, "/api/chat", `{"prompt":"你好"}`)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", w.Code)
	}
}
