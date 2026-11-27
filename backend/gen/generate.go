// Package gen holds code generated from api/openapi.yaml.
package gen

//go:generate go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen -config oapi-codegen.yaml ../../api/openapi.yaml

//go:generate go run github.com/atombender/go-jsonschema --package gen --only-models --capitalization ID --output content.gen.go ../../api/schemas/lesson.schema.json ../../api/schemas/quiz.schema.json ../../api/schemas/flashcard.schema.json ../../api/schemas/review.schema.json ../../api/schemas/check-result.schema.json
