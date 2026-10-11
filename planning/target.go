// story: e16s01
package planning

import "github.com/upareal/upareal/budget"

// TargetStatus says where a category stands against its savings target.
type TargetStatus string

// Statuses in the order they are checked.
const (
	TargetReached TargetStatus = "reached"  // Disponível >= target
	TargetOverdue TargetStatus = "overdue"  // past the due month, not reached
	TargetOnTrack TargetStatus = "on_track" // the month's amount covers Needed
	TargetBehind  TargetStatus = "behind"   // the month's amount is below Needed
)

// TargetPlan is a category's savings target in a month (P4: never stored).
type TargetPlan struct {
	Amount   int64 // the target
	Due      budget.Month
	Needed   int64 // to put in this month to reach Amount by Due; 0 when reached or overdue
	Progress int   // Available / Amount, percent clamped to 0-100
	Status   TargetStatus
}

// planTarget computes the target of category c in month m. Needed splits what
// is missing at the start of the month (target - carry-over) over the months
// left, rounded up so the user never ends short.
func planTarget(t budget.Target, c CategoryPlan, m budget.Month) *TargetPlan {
	tp := &TargetPlan{Amount: t.Amount, Due: t.Due, Progress: int(min(max(c.Available, 0)*100/t.Amount, 100))}
	left := monthsBetween(m, t.Due) + 1
	switch {
	case c.Available >= t.Amount:
		tp.Status, tp.Progress = TargetReached, 100
	case left <= 0:
		tp.Status = TargetOverdue
	default:
		if missing := t.Amount - c.CarryOver; missing > 0 {
			tp.Needed = (missing + left - 1) / left
		}
		tp.Status = TargetBehind
		if c.Amount >= tp.Needed {
			tp.Status = TargetOnTrack
		}
	}
	return tp
}

// monthsBetween is the number of months from a to b (negative when b is before a).
func monthsBetween(a, b budget.Month) int64 {
	as, bs := a.Start(), b.Start()
	return int64(bs.Year()-as.Year())*12 + int64(bs.Month()-as.Month())
}
