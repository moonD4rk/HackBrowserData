package outputter

import (
	"os"
	"testing"
)

func TestNewOutPutter(t *testing.T) {
	out := NewOutPutter("json")
	if out == nil {
		t.Error("NewOutPutter() returned nil")
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
