//go:build tools

package backend

import (
	_ "entgo.io/ent/cmd/ent"
	_ "github.com/atombender/go-jsonschema"
	_ "github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen"
)
