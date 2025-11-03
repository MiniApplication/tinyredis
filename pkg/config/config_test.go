package config

import "testing"

func TestParseJoinAddrs(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: nil,
		},
		{
			name:     "single address",
			input:    "127.0.0.1:17000",
			expected: []string{"127.0.0.1:17000"},
		},
		{
			name:  "multiple addresses with duplicates",
			input: "127.0.0.1:17000, 127.0.0.2:17000;127.0.0.1:17000 127.0.0.3:17000",
			expected: []string{
				"127.0.0.1:17000",
				"127.0.0.2:17000",
				"127.0.0.3:17000",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := ParseJoinAddrs(tc.input)
			if len(result) != len(tc.expected) {
				t.Fatalf("expected %d results, got %d (%v)", len(tc.expected), len(result), result)
			}
			for i := range result {
				if result[i] != tc.expected[i] {
					t.Fatalf("expected %v, got %v", tc.expected, result)
				}
			}
		})
	}
}
