// story: e05s03
package budget

import (
	"errors"
	"fmt"
	"time"
	_ "time/tzdata" // America/Sao_Paulo must load everywhere, including tests
)

// ErrInvalidMonth means a month is not a valid YYYY-MM.
var ErrInvalidMonth = errors.New("budget: month must be YYYY-MM")

// saoPaulo is where months are counted (B3).
var saoPaulo = func() *time.Location {
	loc, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		panic(err) // zone data is embedded; failure is a build bug
	}
	return loc
}()

// Month is a calendar month in America/Sao_Paulo, e.g. "2026-01".
type Month struct{ t time.Time } // first day, 00:00 São Paulo

// ParseMonth validates "YYYY-MM".
func ParseMonth(s string) (Month, error) {
	if len(s) != len("2006-01") {
		return Month{}, fmt.Errorf("%w: %q", ErrInvalidMonth, s)
	}
	t, err := time.ParseInLocation("2006-01", s, saoPaulo)
	if err != nil {
		return Month{}, fmt.Errorf("%w: %q", ErrInvalidMonth, s)
	}
	return Month{t}, nil
}

// MonthOf returns the São Paulo calendar month containing instant t.
func MonthOf(t time.Time) Month {
	sp := t.In(saoPaulo)
	return Month{time.Date(sp.Year(), sp.Month(), 1, 0, 0, 0, 0, saoPaulo)}
}

func (m Month) String() string { return m.t.Format("2006-01") }

// Prev and Next return the adjacent months.
func (m Month) Prev() Month { return Month{m.t.AddDate(0, -1, 0)} }
func (m Month) Next() Month { return Month{m.t.AddDate(0, 1, 0)} }

// Start returns the first instant of the month (00:00 São Paulo).
func (m Month) Start() time.Time { return m.t }
