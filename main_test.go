package main

import "testing"

func TestBoolToFloat(t *testing.T) {
	tests := []struct {
		input    bool
		expected float64
	}{
		{true, 1.0},
		{false, 0.0},
	}

	for _, tt := range tests {
		result := boolToFloat(tt.input)
		if result != tt.expected {
			t.Errorf("Expected %v, got %v", tt.expected, result)
		}
	}
}
