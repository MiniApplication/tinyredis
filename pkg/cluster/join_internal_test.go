package cluster

import (
	"bytes"
	"net/http"
	"testing"
)

func TestBuildJoinURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "plain host",
			input:    "127.0.0.1:17000",
			expected: "http://127.0.0.1:17000/join",
		},
		{
			name:     "http scheme with path",
			input:    "http://127.0.0.1:17000/api",
			expected: "http://127.0.0.1:17000/api/join",
		},
		{
			name:     "https scheme",
			input:    "https://127.0.0.1:17000",
			expected: "https://127.0.0.1:17000/join",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out, err := buildJoinURL(tc.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if out != tc.expected {
				t.Fatalf("expected %s, got %s", tc.expected, out)
			}
		})
	}
}

func TestParseLeaderHint(t *testing.T) {
	resp := &http.Response{Header: make(http.Header)}
	resp.Header.Set("X-Raft-Leader", "127.0.0.1:17000")
	if hint := parseLeaderHint(resp, ""); hint != "127.0.0.1:17000" {
		t.Fatalf("expected header hint, got %q", hint)
	}

	resp = &http.Response{Header: make(http.Header)}
	resp.Header.Set("Location", "http://127.0.0.1:17000/join")
	if hint := parseLeaderHint(resp, ""); hint != "http://127.0.0.1:17000" {
		t.Fatalf("expected location hint, got %q", hint)
	}

	if hint := parseLeaderHint(nil, "not leader (leader=127.0.0.1:17000)"); hint != "127.0.0.1:17000" {
		t.Fatalf("expected body hint, got %q", hint)
	}
}

func TestNormalizeJoinAddr(t *testing.T) {
	tests := map[string]string{
		"http://127.0.0.1:17000":  "127.0.0.1:17000",
		"https://127.0.0.1:17000": "127.0.0.1:17000",
		"127.0.0.1:17000/":        "127.0.0.1:17000",
		" 127.0.0.1:17000 ":       "127.0.0.1:17000",
	}

	for in, expected := range tests {
		if out := normalizeJoinAddr(in); out != expected {
			t.Fatalf("expected %q -> %q, got %q", in, expected, out)
		}
	}
}

func TestRaftLogWriterFiltersRollback(t *testing.T) {
	var buf bytes.Buffer
	writer := newRaftLogWriter(&buf)

	filtered := []byte("Rollback failed: tx closed")
	if n, err := writer.Write(filtered); err != nil || n != len(filtered) {
		t.Fatalf("unexpected result from filtered write: n=%d err=%v", n, err)
	}
	if buf.Len() != 0 {
		t.Fatalf("expected filtered message to be dropped, got %s", buf.String())
	}

	allowed := []byte("regular log line")
	if n, err := writer.Write(allowed); err != nil || n != len(allowed) {
		t.Fatalf("unexpected result from allowed write: n=%d err=%v", n, err)
	}
	if buf.String() != string(allowed) {
		t.Fatalf("expected buffer to contain %q, got %q", allowed, buf.String())
	}
}
