package packfiles

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestHandler(t *testing.T) {
	dir := t.TempDir()
	body := []byte("SQLite format 3\x00rest-of-file")
	if err := os.WriteFile(filepath.Join(dir, "bible-SUV-v1.sqlite"), body, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "secret.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.Handle("GET /packs/{file}", Handler(dir))
	ts := httptest.NewServer(mux)
	defer ts.Close()

	get := func(path string, hdr map[string]string) (*http.Response, []byte) {
		t.Helper()
		req, _ := http.NewRequest(http.MethodGet, ts.URL+path, nil)
		for k, v := range hdr {
			req.Header.Set(k, v)
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		b, _ := io.ReadAll(res.Body)
		return res, b
	}

	res, b := get("/packs/bible-SUV-v1.sqlite", nil)
	if res.StatusCode != 200 || string(b) != string(body) || res.Header.Get("Content-Type") != "application/vnd.sqlite3" {
		t.Fatalf("full: %d %q %s", res.StatusCode, b, res.Header.Get("Content-Type"))
	}
	etag := res.Header.Get("ETag")

	res, b = get("/packs/bible-SUV-v1.sqlite", map[string]string{"Range": "bytes=0-5"})
	if res.StatusCode != http.StatusPartialContent || string(b) != "SQLite" {
		t.Fatalf("range: %d %q", res.StatusCode, b)
	}
	if res, _ = get("/packs/bible-SUV-v1.sqlite", map[string]string{"If-None-Match": etag}); res.StatusCode != http.StatusNotModified {
		t.Fatalf("etag: %d", res.StatusCode)
	}
	for _, p := range []string{"/packs/secret.txt", "/packs/..%2Fsecret.txt", "/packs/bible-SUV-v2.sqlite", "/packs/.bible-SUV-v1.sqlite.tmp", "/packs/bible-SUV-v0.sqlite"} {
		if res, _ := get(p, nil); res.StatusCode != http.StatusNotFound {
			t.Fatalf("%s: %d, want 404", p, res.StatusCode)
		}
	}
}
