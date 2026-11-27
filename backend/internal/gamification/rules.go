// Package gamification holds the XP, level and daily goal rules.
package gamification

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Rules are the gamification rules loaded from api/gamification.yaml.
type Rules struct {
	XpPerLesson      int `yaml:"xpPerLesson"`
	XpPerLevel       int `yaml:"xpPerLevel"`
	DailyGoalLessons int `yaml:"dailyGoalLessons"`
}

// Load reads and validates the rules file at path.
func Load(path string) (Rules, error) {
	data, err := os.ReadFile(path) //nolint:gosec // path comes from the operator's configuration
	if err != nil {
		return Rules{}, fmt.Errorf("read rules: %w", err)
	}
	var r Rules
	if err := yaml.Unmarshal(data, &r); err != nil {
		return Rules{}, fmt.Errorf("parse rules: %w", err)
	}
	if r.XpPerLesson < 1 || r.XpPerLevel < 1 || r.DailyGoalLessons < 1 {
		return Rules{}, fmt.Errorf("rules in %s must be positive numbers", path)
	}
	return r, nil
}
