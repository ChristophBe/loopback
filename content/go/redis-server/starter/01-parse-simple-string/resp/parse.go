package resp

import "errors"

// Parse returns the text of a RESP simple string such as "+OK\r\n".
// It returns an error when the input has no "+" prefix.
func Parse(input string) (string, error) {
	return "", errors.New("not implemented")
}
