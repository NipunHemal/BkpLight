package config

import (
	"bytes"
	"os"
	"testing"
)

func FuzzDecode(f *testing.F) {
	raw, err := os.ReadFile("testdata/valid.yaml")
	if err != nil {
		f.Fatal(err)
	}
	for _, seed := range [][]byte{raw, {}, []byte("version: 1\nversion: 1"), []byte("version: &a 1\nx: *a")} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, raw []byte) {
		_, err := Decode(bytes.NewReader(raw))
		if len(raw) > 1<<20 && err == nil {
			t.Fatal("oversized input admitted")
		}
	})
}
