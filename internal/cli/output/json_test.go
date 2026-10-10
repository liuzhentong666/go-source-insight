package output

import (
	"encoding/json"
	"testing"
)

func TestJSONFormatterPreservesStructuredResults(t *testing.T) {
	raw := NewJSONFormatter().Format("{\"files\":[{\"path\":\"main.go\"}]}")
	var v struct {
		Success bool
		Result  struct{ Files []struct{ Path string } }
	}
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		t.Fatal(err)
	}
	if !v.Success || len(v.Result.Files) != 1 || v.Result.Files[0].Path != "main.go" {
		t.Fatalf("wrong envelope: %s", raw)
	}
}
func TestJSONFormatterEscapesPlainText(t *testing.T) {
	raw := NewJSONFormatter().Format("line1\n\"line2\"")
	var v struct{ Result string }
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		t.Fatal(err)
	}
	if v.Result != "line1\n\"line2\"" {
		t.Fatalf("text lost: %s", raw)
	}
}
