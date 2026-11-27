package resp

import "testing"

func TestParseSimpleString(t *testing.T) {
	got, err := Parse("+OK\r\n")
	if err != nil {
		t.Fatal(err)
	}
	if got != "OK" {
		t.Errorf("Parse = %q, want %q", got, "OK")
	}
}

func TestParseSimpleStringMissingPrefix(t *testing.T) {
	if _, err := Parse("OK\r\n"); err == nil {
		t.Error("want an error for input without the + prefix")
	}
}
