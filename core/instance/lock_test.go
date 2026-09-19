package instance

import "testing"

func TestExclusiveAndRelease(t *testing.T) {
	dir := t.TempDir()
	first, err := Acquire(dir)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Acquire(dir)
	if err == nil {
		second.Close()
		first.Close()
		t.Fatal("two owners")
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	third, err := Acquire(dir)
	if err != nil {
		t.Fatal(err)
	}
	third.Close()
}
