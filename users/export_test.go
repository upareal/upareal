// story: e03s02
package users

import "time"

// SetNow replaces the service clock in tests (compiled only into tests).
func SetNow(s *Service, now func() time.Time) { s.now = now }
