package codesignalcli

import (
	"io/fs"
	"testing"
	"testing/fstest"
)

func TestReadRecorderCountsOnlyContentReadsAsReads(t *testing.T) {
	inner := fstest.MapFS{
		"opened/tsconfig.json":   {Data: []byte("{}")},
		"read/tsconfig.json":     {Data: []byte("{}")},
		"stat/tsconfig.json":     {Data: []byte("{}")},
		"listed/package.json":    {Data: []byte("{}")},
		"untouched/package.json": {Data: []byte("{}")},
	}
	recorder := newReadRecorder(inner)

	file, err := recorder.Open("opened/tsconfig.json")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	_ = file.Close()
	if _, err := recorder.ReadFile("read/tsconfig.json"); err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if _, err := fs.Stat(recorder, "stat/tsconfig.json"); err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if _, err := recorder.ReadDir("listed"); err != nil {
		t.Fatalf("ReadDir: %v", err)
	}

	cases := []struct {
		name string
		want bool
	}{
		{"opened/tsconfig.json", true},
		{"read/tsconfig.json", true},
		{"stat/tsconfig.json", false},
		{"listed/package.json", false},
		{"untouched/package.json", false},
	}
	for _, c := range cases {
		if got := recorder.wasOpened(c.name); got != c.want {
			t.Errorf("wasOpened(%q) = %v, want %v", c.name, got, c.want)
		}
	}
}
