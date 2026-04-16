package integration

import (
	"context"
	"fmt"
	"strings"
)

// BarsukasIntegration handles @barsukas mentions.
// Supported query forms:
//   - @barsukas <word>          — look up lemma (info)
//   - @barsukas info <word>     — same as above
//   - @barsukas search <query>  — show similar matches
//   - @barsukas define <word>   — LLM-generated definition and POS
type BarsukasIntegration struct {
	client *BarsukasClient
}

func NewBarsukasIntegration(baseURL string) *BarsukasIntegration {
	return &BarsukasIntegration{client: NewBarsukasClient(baseURL)}
}

func (b *BarsukasIntegration) Name() string { return "barsukas" }

func (b *BarsukasIntegration) Handle(ctx context.Context, query string) (string, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return "[barsukas] Usage: @barsukas <word> | info <word> | search <query> | define <word>", nil
	}

	// Split off optional subcommand.
	parts := strings.SplitN(query, " ", 2)
	sub := strings.ToLower(parts[0])
	rest := ""
	if len(parts) == 2 {
		rest = strings.TrimSpace(parts[1])
	}

	switch sub {
	case "define":
		if rest == "" {
			return "[barsukas] Usage: @barsukas define <word>", nil
		}
		return b.handleDefine(ctx, rest)
	case "search":
		if rest == "" {
			return "[barsukas] Usage: @barsukas search <query>", nil
		}
		return b.handleSearch(ctx, rest)
	case "info":
		if rest == "" {
			return "[barsukas] Usage: @barsukas info <word>", nil
		}
		return b.handleInfo(ctx, rest)
	default:
		// Bare word — treat as info lookup.
		return b.handleInfo(ctx, query)
	}
}

func (b *BarsukasIntegration) handleInfo(ctx context.Context, word string) (string, error) {
	res, err := b.client.CheckLemmaExists(ctx, word, "")
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	if res.ExactMatch != nil {
		m := res.ExactMatch
		sb.WriteString(fmt.Sprintf("[barsukas] \"%s\" (%s) — %s", m.LemmaText, m.PosType, m.DefinitionText))
		if len(res.SimilarMatches) > 0 {
			similar := similarNames(res.SimilarMatches)
			sb.WriteString(fmt.Sprintf("\nSimilar: %s", strings.Join(similar, ", ")))
		}
	} else if len(res.SimilarMatches) > 0 {
		similar := similarNames(res.SimilarMatches)
		sb.WriteString(fmt.Sprintf("[barsukas] \"%s\" not found. Similar: %s", word, strings.Join(similar, ", ")))
	} else {
		sb.WriteString(fmt.Sprintf("[barsukas] \"%s\" not found.", word))
	}
	return sb.String(), nil
}

func (b *BarsukasIntegration) handleSearch(ctx context.Context, query string) (string, error) {
	res, err := b.client.CheckLemmaExists(ctx, query, "")
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("[barsukas] Search results for \"%s\":", query))
	if res.ExactMatch != nil {
		m := res.ExactMatch
		sb.WriteString(fmt.Sprintf("\n  [exact] %s (%s) — %s", m.LemmaText, m.PosType, m.DefinitionText))
	}
	for _, m := range res.SimilarMatches {
		sb.WriteString(fmt.Sprintf("\n  %s (%s) — %s", m.LemmaText, m.PosType, m.DefinitionText))
	}
	if res.ExactMatch == nil && len(res.SimilarMatches) == 0 {
		sb.WriteString("\n  No results found.")
	}
	return sb.String(), nil
}

func (b *BarsukasIntegration) handleDefine(ctx context.Context, word string) (string, error) {
	res, err := b.client.AutoPopulateLemma(ctx, word)
	if err != nil {
		return "", err
	}

	posInfo := res.PosType
	if res.PosSubtype != "" {
		posInfo = fmt.Sprintf("%s/%s", res.PosType, res.PosSubtype)
	}
	out := fmt.Sprintf("[barsukas] \"%s\" (%s) — %s", word, posInfo, res.Definition)
	if res.SuggestedDifficulty > 0 {
		out += fmt.Sprintf(" [difficulty: %d]", res.SuggestedDifficulty)
	}
	return out, nil
}

func similarNames(matches []LemmaSummary) []string {
	names := make([]string, 0, len(matches))
	for _, m := range matches {
		names = append(names, m.LemmaText)
	}
	return names
}
