package tools

import (
	"testing"
)

func TestNewComplexityAnalyzer(t *testing.T) {
	analyzer := NewComplexityAnalyzer()

	if analyzer == nil {
		t.Error("NewComplexityAnalyzer() returned nil")
	}

	if analyzer.Name() != "complexity_analyzer" {
		t.Errorf("Expected name 'complexity_analyzer', got '%s'", analyzer.Name())
	}
}


