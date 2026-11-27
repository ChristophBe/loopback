package auth

import "time"

// SetClock replaces the clock of the service in tests.
func SetClock(s *Service, now func() time.Time) { s.now = now }
