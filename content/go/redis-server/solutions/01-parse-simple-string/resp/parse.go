package resp

import (
	"errors"
	"strings"
)

// Parse returns the text of a RESP simple string such as "+OK\r\n".
func Parse(input string) (string, error) {
	if !strings.HasPrefix(input, "+") {
		return "", errors.New("resp: simple string must start with '+'")
	}
	text, ok := strings.CutSuffix(input[1:], "\r\n")
	if !ok {
		return "", errors.New("resp: simple string must end with CRLF")
	}
	return text, nil
}
