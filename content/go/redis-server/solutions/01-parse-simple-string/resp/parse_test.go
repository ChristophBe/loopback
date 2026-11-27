package resp

import "testing"

func TestParseSimpleString(t *testing.T) {
	got, err := Parse("+OK\r\n")
	if err != nil {
		t.Fatalf("Parse returned an error: %v", err)
	}
	if got != "OK" {
		t.Fatalf("Parse = %q, want %q", got, "OK")
	}
}

func TestParseSimpleStringMissingPrefix(t *testing.T) {
	if _, err := Parse("OK\r\n"); err == nil {
		t.Fatal("Parse accepted input without a + prefix")
	}
}
