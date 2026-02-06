package tools

import (
	"testing"
)

func TestNewTestGenerator(t *testing.T) {
	logger := NewNoopLogger()
	generator := NewTestGenerator(logger)

	if generator == nil {
		t.Error("NewTestGenerator() returned nil")
	}

	if generator.Name() != "test_generator" {
		t.Errorf("Expected name 'test_generator', got '%s'", generator.Name())
	}
}

func TestValidate(t *testing.T) {
	logger := NewNoopLogger()
	generator := NewTestGenerator(logger)

	tests := []struct {
		name    string
		input   any
		wantErr bool
	}{
		{
			name:    "valid request with function name",
			input:   GenerateRequest{FunctionName: "TestFunc"},
			wantErr: false,
		},
		{
			name:    "valid request with file path",
			input:   GenerateRequest{FilePath: "/etc/hosts"},
			wantErr: false,
		},
		{
			name:    "invalid request - no target",
			input:   GenerateRequest{},
			wantErr: true,
		},
		{
			name:    "invalid input type",
			input:   "string",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := generator.Validate(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}


