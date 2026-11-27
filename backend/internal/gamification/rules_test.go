package gamification_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ChristophBe/loopback/backend/internal/gamification"
)

func TestLoadRepoRules(t *testing.T) {
	r, err := gamification.Load("../../../api/gamification.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if r.XpPerLesson != 50 {
		t.Errorf("XpPerLesson = %d, want 50", r.XpPerLesson)
	}
}

func TestLoadRejectsNonPositive(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rules.yaml")
	if err := os.WriteFile(path, []byte("xpPerLesson: 0\nxpPerLevel: 1\ndailyGoalLessons: 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := gamification.Load(path); err == nil {
		t.Fatal("want an error for xpPerLesson 0")
	}
}
