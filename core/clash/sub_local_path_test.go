package clash

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveSubInput_LocalTxtWithURL(t *testing.T) {
	dir := t.TempDir()
	// Cyrillic filename like the user's Desktop file
	p := filepath.Join(dir, "Новый текстовый документ.txt")
	want := "https://raw.githubusercontent.com/barry-far/V2ray-config/main/Sub1.txt"
	if err := os.WriteFile(p, []byte(want+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	remote, body, err := resolveSubInput(p)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if len(body) != 0 {
		t.Fatalf("expected remote URL, got body len=%d", len(body))
	}
	if remote != want {
		t.Fatalf("got %q want %q", remote, want)
	}
}

func TestResolveSubInput_DirectHTTPS(t *testing.T) {
	u := "https://example.com/sub"
	remote, body, err := resolveSubInput(u)
	if err != nil {
		t.Fatal(err)
	}
	if remote != u || body != nil {
		t.Fatalf("remote=%q body=%v", remote, body)
	}
}

func TestResolveSubInput_MissingPath(t *testing.T) {
	_, _, err := resolveSubInput(`C:\no\such\file\sub.txt`)
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestFirstHTTPURL_Quoted(t *testing.T) {
	got := firstHTTPURL(`"https://x.example/a"`)
	if got != "https://x.example/a" {
		t.Fatalf("got %q", got)
	}
}
