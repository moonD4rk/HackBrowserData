package browingdata

import (
	"os"
	"testing"
)

func TestNewOutPutter(t *testing.T) {
	t.Parallel()
	out := NewOutPutter("json")
	if out == nil {
		t.Error("New() returned nil")
	}
	f, err := out.CreateFile("results", "test.json")
	if err != nil {
		t.Error("CreateFile() returned an error", err)
	}
	defer os.RemoveAll("results")
	err = out.Write(nil, f)
	if err != nil {
		t.Error("Write() returned an error", err)
	}
}
