// Package content validates the lesson content under content/ and imports it
// into the database.
package content

import (
	"fmt"
	"strings"
)

// Problem is one validation failure, located by file and field.
type Problem struct {
	File    string
	Field   string
	Message string
}

func (p Problem) String() string {
	return fmt.Sprintf("%s: %s %s", p.File, p.Field, p.Message)
}

// Problems is the list of every validation failure found in a content tree.
// It is an error so that callers can return it directly.
type Problems []Problem

func (ps Problems) Error() string {
	lines := make([]string, len(ps))
	for i, p := range ps {
		lines[i] = p.String()
	}
	return fmt.Sprintf("%d content problem(s):\n%s", len(ps), strings.Join(lines, "\n"))
}
