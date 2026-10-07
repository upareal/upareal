// story: e04s01
package ledger

// System account roles. Every user has exactly one account per role; the app
// relies on them, so they cannot be deleted or archived (invariant L7).
const (
	roleOpeningBalance       = "opening_balance"       // balances opening balances and manual adjustments
	roleUncategorizedIncome  = "uncategorized_income"  // second leg of imported inflows until assigned
	roleUncategorizedExpense = "uncategorized_expense" // second leg of imported outflows until assigned
)

var systemAccounts = []struct {
	role, accountType, name string
}{
	{roleOpeningBalance, "equity", "Saldo inicial"},
	{roleUncategorizedIncome, "income", "A categorizar"},
	{roleUncategorizedExpense, "expense", "A categorizar"},
}
