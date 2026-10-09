// story: e14s02
package ledger

import (
	"context"
	"fmt"
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// Suggestion is the counter account suggested for a queued line, with the
// number of similar past lines that used it. It is only a suggestion: it
// never writes; the user decides.
type Suggestion struct {
	AccountID, AccountName string
	Matches                int
}

const minSimilarity = 0.5 // Jaccard over description tokens

// bankNoise are generic statement words that would make unrelated lines look alike.
var bankNoise = map[string]bool{
	"pagto": true, "pagamento": true, "eletron": true, "cobranca": true, "compra": true, "cartao": true,
	"debito": true, "credito": true, "pix": true, "ted": true, "doc": true, "transferencia": true,
	"enviado": true, "recebido": true, "rem": true, "des": true, "ltda": true, "com": true, "www": true,
}

type pastLine struct {
	tokens    map[string]bool
	accountID string
	out       bool // money left (counter is an expense)
}

// SuggestCounters suggests, for each queued line, the active income/expense
// account the user chose for similar past lines (same direction), from
// their 2000 most recent simple (two-leg) transactions. Keyed by
// transaction ID; lines with nothing similar are absent. Computed in memory,
// never stored.
func (s *Service) SuggestCounters(ctx context.Context, userID string, lines []QueuedLine) (map[string]Suggestion, error) {
	out := map[string]Suggestion{}
	if len(lines) == 0 {
		return out, nil
	}
	rows, err := s.q.SuggestionHistory(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("ledger: suggestion history: %w", err)
	}
	dk, err := s.userKey(ctx, s.q, userID)
	if err != nil {
		return nil, err
	}
	// Group legs per transaction; keep money-leg + one usable counter leg.
	type leg struct {
		typ, accountID string
		amount         int64
		excluded       bool
	}
	legs := map[string][]leg{}
	sealed := map[string][]byte{}
	var order []string
	for _, r := range rows {
		if _, seen := legs[r.ID]; !seen {
			order = append(order, r.ID)
			sealed[r.ID] = r.Description
		}
		legs[r.ID] = append(legs[r.ID], leg{r.Type, r.AccountID, r.Amount, r.Excluded == 1})
	}
	var past []pastLine
	for _, id := range order {
		ls := legs[id]
		if len(ls) != 2 {
			continue // splits and odd shapes do not teach a single account
		}
		money, counter := ls[0], ls[1]
		if counter.typ == "asset" || counter.typ == "liability" {
			money, counter = counter, money
		}
		if (money.typ != "asset" && money.typ != "liability") || (counter.typ != "income" && counter.typ != "expense") || counter.excluded {
			continue
		}
		desc, err := dk.Open(fieldDescription, sealed[id])
		if err != nil {
			return nil, err
		}
		if tokens := descriptionTokens(desc); len(tokens) > 0 {
			past = append(past, pastLine{tokens, counter.accountID, counter.typ == "expense"})
		}
	}
	names, err := s.activeNames(ctx, userID)
	if err != nil {
		return nil, err
	}
	for _, l := range lines {
		q := descriptionTokens(l.Description)
		if len(q) == 0 {
			continue
		}
		count, best := map[string]int{}, map[string]float64{}
		for _, p := range past {
			if p.out != (l.Amount < 0) {
				continue
			}
			if sim := jaccard(q, p.tokens); sim >= minSimilarity {
				count[p.accountID]++
				best[p.accountID] = max(best[p.accountID], sim)
			}
		}
		var pick string
		for id := range count {
			if pick == "" || count[id] > count[pick] || (count[id] == count[pick] && (best[id] > best[pick] || (best[id] == best[pick] && names[id] < names[pick]))) {
				pick = id
			}
		}
		if pick != "" {
			out[l.TransactionID] = Suggestion{AccountID: pick, AccountName: names[pick], Matches: count[pick]}
		}
	}
	return out, nil
}

// activeNames maps userID's active accounts to their names.
func (s *Service) activeNames(ctx context.Context, userID string) (map[string]string, error) {
	accounts, err := s.Accounts(ctx, userID)
	if err != nil {
		return nil, err
	}
	names := make(map[string]string, len(accounts))
	for _, a := range accounts {
		names[a.ID] = a.Name
	}
	return names, nil
}

var stripAccents = transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)

// descriptionTokens lowercases, removes accents, splits on anything that is
// not a letter, and drops short tokens and bank boilerplate.
func descriptionTokens(desc string) map[string]bool {
	plain, _, err := transform.String(stripAccents, strings.ToLower(desc))
	if err != nil {
		plain = strings.ToLower(desc)
	}
	tokens := map[string]bool{}
	for _, w := range strings.FieldsFunc(plain, func(r rune) bool { return !unicode.IsLetter(r) }) {
		if len([]rune(w)) >= 3 && !bankNoise[w] {
			tokens[w] = true
		}
	}
	return tokens
}

func jaccard(a, b map[string]bool) float64 {
	inter := 0
	for w := range a {
		if b[w] {
			inter++
		}
	}
	union := len(a) + len(b) - inter
	if union == 0 {
		return 0
	}
	return float64(inter) / float64(union)
}
