package gen_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/ChristophBe/loopback/backend/gen"
)

func loadExample(t *testing.T, name string, v any) {
	t.Helper()
	data, err := os.ReadFile("../../api/schemas/examples/" + name)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, v); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
}

func TestContentTypesDecodeValidExamples(t *testing.T) {
	var quiz gen.QuizSchemaJson
	loadExample(t, "quiz-valid.json", &quiz)
	if len(quiz.Questions) == 0 {
		t.Error("quiz has no questions")
	}

	var cards gen.FlashcardSchemaJson
	loadExample(t, "flashcard-valid.json", &cards)
	if len(cards.Cards) == 0 {
		t.Error("flashcard has no cards")
	}

	var review gen.ReviewSchemaJson
	loadExample(t, "review-valid.json", &review)
	if len(review.FlaggedLines) == 0 {
		t.Error("review has no flagged lines")
	}
}
