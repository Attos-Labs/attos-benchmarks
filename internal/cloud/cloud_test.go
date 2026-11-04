package cloud

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	attos "github.com/Attos-Labs/attos-go"
)

// fake mimics the two cloud endpoints the benchmarks use (compile + sync) so the
// upload -> compile -> sync -> open path is tested without an account or network.
func fake(t *testing.T) *httptest.Server {
	dir := t.TempDir()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/datasets/compile", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-API-Key") != "good" {
			http.Error(w, `{"error":"bad key"}`, 401)
			return
		}
		csv := filepath.Join(dir, "in.csv")
		f, _ := os.Create(csv)
		fmt.Fprintln(f, "k,attos_value")
		sc := bufio.NewScanner(r.Body)
		n := 0
		for sc.Scan() {
			var kv struct{ Key, Value string }
			json.Unmarshal(sc.Bytes(), &kv)
			fmt.Fprintf(f, "%s,%s\n", kv.Key, kv.Value)
			n++
		}
		f.Close()
		b := attos.NewBuilder()
		if err := b.BuildFromCSV(csv); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		b.Compile(filepath.Join(dir, "abc.attos"))
		w.WriteHeader(201)
		fmt.Fprintf(w, `{"id":"abc","row_count":%d,"status":"ready"}`, n)
	})
	mux.HandleFunc("GET /api/v1/datasets/sync/abc", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-API-Key") != "good" {
			http.Error(w, "no", 401)
			return
		}
		w.Header().Set("ETag", `"v1"`)
		http.ServeFile(w, r, filepath.Join(dir, "abc.attos"))
	})
	return httptest.NewServer(mux)
}

func TestUploadFetchOpen(t *testing.T) {
	srv := fake(t)
	defer srv.Close()
	c := &Client{Key: "good", BaseURL: srv.URL, HTTP: srv.Client()}
	nd := "{\"key\":\"user:a\",\"value\":\"1\"}\n{\"key\":\"user:b\",\"value\":\"2\"}\n"
	id, err := c.Upload(context.Background(), "t", stringsReader(nd))
	if err != nil || id != "abc" {
		t.Fatalf("upload: %q %v", id, err)
	}
	p, err := c.Fetch(context.Background(), id, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m, err := attos.Open(p)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	if v, err := m.GetString("user:b"); err != nil || v != "2" {
		t.Fatalf("got %q %v", v, err)
	}
}

func TestBadKey(t *testing.T) {
	srv := fake(t)
	defer srv.Close()
	c := &Client{Key: "nope", BaseURL: srv.URL, HTTP: srv.Client()}
	if _, err := c.Upload(context.Background(), "t", stringsReader("{}")); err == nil {
		t.Fatal("expected 401 error")
	}
}
