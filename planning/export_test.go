package planning

import "time"

// SetClock fixes the service's clock in tests.
func SetClock(s *Service, now func() time.Time) { s.now = now }
