package runtimeassets

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestVersionProbeIdentityAndExit(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fixtures")
	}
	d := t.TempDir()
	a, b := filepath.Join(d, "a"), filepath.Join(d, "b")
	write := func(p, body string) {
		t.Helper()
		if err := os.WriteFile(p, []byte("#!/bin/sh\n"+body), 0755); err != nil {
			t.Fatal(err)
		}
	}
	write(a, "echo Mihomo v1.19.27\n")
	write(b, "echo Mihomo v1.19.31\n")
	if v, e := readCoreVersion(a); e != nil || v != "v1.19.27" {
		t.Fatalf("a: %s %v", v, e)
	}
	if v, e := readCoreVersion(b); e != nil || v != "v1.19.31" {
		t.Fatalf("b: %s %v", v, e)
	}
	write(a, "echo Mihomo v1.19.32\n")
	if v, e := readCoreVersion(a); e != nil || v != "v1.19.32" {
		t.Fatalf("replacement: %s %v", v, e)
	}
	write(b, "echo Mihomo v1.19.33; exit 1\n")
	if _, e := readCoreVersion(b); e == nil {
		t.Fatal("failed process accepted")
	}
	write(b, "echo unknown\n")
	if _, e := readCoreVersion(b); e == nil {
		t.Fatal("unknown version accepted")
	}
}
