package integration

import (
	"context"
	"fmt"
	"strings"
)

// BarsukasIntegration handles @barsukas mentions.
//
// Commands:
//
//	@barsukas <word>                   — search for a word, show top result with GUID
//	@barsukas info <word>              — same as above
//	@barsukas search <query>           — list up to 5 results with GUIDs
//	@barsukas translate <lang> <word>  — search for word, show translation in <lang>
//	@barsukas forms <lang> <guid>      — inflected forms for a GUID in <lang>
//	@barsukas grammar <lang> <guid>    — grammar facts for a GUID in <lang>
//	@barsukas sentences <lang> <guid>  — example sentences for a GUID in <lang>
//	@barsukas help                     — list all commands
//
// Language codes: en zh fr lt ko es de pt sw vi
type BarsukasIntegration struct {
	client *BarsukasClient
}

func NewBarsukasIntegration(baseURL string) *BarsukasIntegration {
	return &BarsukasIntegration{client: NewBarsukasClient(baseURL)}
}

func (b *BarsukasIntegration) Name() string { return "barsukas" }

var helpText = "[barsukas] Commands:\n" +
	"  @barsukas <word>                   search for a word (shows GUID)\n" +
	"  @barsukas info <word>              same as above\n" +
	"  @barsukas search <query>           list up to 5 results with GUIDs\n" +
	"  @barsukas translate <lang> <word>  translation of <word> into <lang>\n" +
	"  @barsukas forms <lang> <guid>      inflected forms for a GUID in <lang>\n" +
	"  @barsukas grammar <lang> <guid>    grammar facts for a GUID in <lang>\n" +
	"  @barsukas sentences <lang> <guid>  example sentences for a GUID in <lang>\n" +
	"  @barsukas help                     show this message\n" +
	"Language codes: " + strings.Join(SupportedLanguages, " ")

func (b *BarsukasIntegration) Handle(ctx context.Context, query string) (string, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return helpText, nil
	}

	parts := strings.SplitN(query, " ", 2)
	sub := strings.ToLower(parts[0])
	rest := ""
	if len(parts) == 2 {
		rest = strings.TrimSpace(parts[1])
	}

	switch sub {
	case "help":
		return helpText, nil
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
	case "translate":
		lang, word, ok := splitLangArg(rest)
		if !ok {
			return "[barsukas] Usage: @barsukas translate <lang> <word>  (e.g. translate lt dog)", nil
		}
		if msg := checkLang(lang); msg != "" {
			return msg, nil
		}
		return b.handleTranslate(ctx, lang, word)
	case "forms":
		lang, guid, ok := splitLangArg(rest)
		if !ok {
			return "[barsukas] Usage: @barsukas forms <lang> <guid>  (e.g. forms lt N01_001)", nil
		}
		if msg := checkLang(lang); msg != "" {
			return msg, nil
		}
		return b.handleForms(ctx, lang, guid)
	case "grammar":
		lang, guid, ok := splitLangArg(rest)
		if !ok {
			return "[barsukas] Usage: @barsukas grammar <lang> <guid>  (e.g. grammar lt N01_001)", nil
		}
		if msg := checkLang(lang); msg != "" {
			return msg, nil
		}
		return b.handleGrammar(ctx, lang, guid)
	case "sentences":
		lang, guid, ok := splitLangArg(rest)
		if !ok {
			return "[barsukas] Usage: @barsukas sentences <lang> <guid>  (e.g. sentences lt V01_001)", nil
		}
		if msg := checkLang(lang); msg != "" {
			return msg, nil
		}
		return b.handleSentences(ctx, lang, guid)
	default:
		// Bare word — treat as info lookup.
		return b.handleInfo(ctx, query)
	}
}

// splitLangArg splits "lang rest" into (lang, rest, true), or returns false if either part is missing.
func splitLangArg(s string) (lang, arg string, ok bool) {
	parts := strings.SplitN(strings.TrimSpace(s), " ", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return strings.ToLower(parts[0]), strings.TrimSpace(parts[1]), true
}

// checkLang returns a non-empty user-facing error string if lang is not valid.
func checkLang(lang string) string {
	if !IsValidLanguage(lang) {
		return fmt.Sprintf("[barsukas] Unknown language code %q. Valid codes: %s",
			lang, strings.Join(SupportedLanguages, " "))
	}
	return ""
}

func (b *BarsukasIntegration) handleInfo(ctx context.Context, word string) (string, error) {
	results, err := b.client.Search(ctx, word, "", 5)
	if err != nil {
		return "", err
	}
	if len(results) == 0 {
		return fmt.Sprintf("[barsukas] \"%s\" not found.", word), nil
	}

	var sb strings.Builder
	// Show the best match as the primary result.
	top := results[0]
	sb.WriteString(formatResult(top))

	// Any additional hits are shown as "also found" without full definitions.
	if len(results) > 1 {
		var others []string
		for _, r := range results[1:] {
			others = append(others, fmt.Sprintf("%s [%s]", r.LemmaText, r.GUID))
		}
		sb.WriteString(fmt.Sprintf("\nAlso: %s", strings.Join(others, ", ")))
	}
	return sb.String(), nil
}

func (b *BarsukasIntegration) handleSearch(ctx context.Context, query string) (string, error) {
	results, err := b.client.Search(ctx, query, "", 5)
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("[barsukas] Search results for \"%s\":", query))
	if len(results) == 0 {
		sb.WriteString("\n  No results found.")
		return sb.String(), nil
	}
	for _, r := range results {
		line := fmt.Sprintf("\n  [%s] %s (%s) — %s", r.GUID, r.LemmaText, r.PosType, r.Definition)
		if r.Disambiguation != "" {
			line += fmt.Sprintf(" (%s)", r.Disambiguation)
		}
		sb.WriteString(line)
	}
	return sb.String(), nil
}

func (b *BarsukasIntegration) handleTranslate(ctx context.Context, lang, word string) (string, error) {
	// Find the lemma first via search.
	results, err := b.client.Search(ctx, word, "", 5)
	if err != nil {
		return "", err
	}
	if len(results) == 0 {
		return fmt.Sprintf("[barsukas] \"%s\" not found.", word), nil
	}

	// First pass: try translations already present in search results (sample of up to 3).
	// If the target language isn't in the sample, fall back to a dedicated translations call.
	var sb strings.Builder
	for _, r := range results {
		// Only show results where the lemma text matches the query — the search
		// API returns fuzzy matches (prefix/contains) which are not useful here.
		// lemma_text may include disambiguation in parens, e.g. "mad (angry)".
		lt := strings.ToLower(r.LemmaText)
		w := strings.ToLower(word)
		if lt != w && !strings.HasPrefix(lt, w+" ") {
			continue
		}
		t := r.Translations[lang]
		if t == "" {
			// Sample didn't include this language; fetch the full translations.
			full, _, err := b.client.GetTranslations(ctx, r.GUID, lang)
			if err == nil {
				t = full[lang]
			}
		}
		if t == "" {
			continue
		}
		sb.WriteString(fmt.Sprintf("\n  [%s] %s (%s) → %s: %s", r.GUID, r.LemmaText, r.PosType, lang, t))
	}

	if sb.Len() == 0 {
		return fmt.Sprintf("[barsukas] No %s translation found for \"%s\".", lang, word), nil
	}
	return fmt.Sprintf("[barsukas] Translations of \"%s\" → %s:%s", word, lang, sb.String()), nil
}

func (b *BarsukasIntegration) handleForms(ctx context.Context, lang, guid string) (string, error) {
	forms, err := b.client.GetForms(ctx, guid, lang)
	if err != nil {
		return "", err
	}
	if len(forms) == 0 {
		return fmt.Sprintf("[barsukas] No %s forms found for %s.", lang, guid), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("[barsukas] Forms of %s in %s:", guid, lang))
	for _, f := range forms {
		base := ""
		if f.IsBaseForm {
			base = " [base]"
		}
		ipa := ""
		if f.IPAPronunciation != "" {
			ipa = fmt.Sprintf(" /%s/", f.IPAPronunciation)
		}
		sb.WriteString(fmt.Sprintf("\n  %s (%s)%s%s", f.FormText, f.GrammaticalForm, base, ipa))
	}
	return sb.String(), nil
}

func (b *BarsukasIntegration) handleGrammar(ctx context.Context, lang, guid string) (string, error) {
	facts, err := b.client.GetGrammar(ctx, guid, lang)
	if err != nil {
		return "", err
	}
	if len(facts) == 0 {
		return fmt.Sprintf("[barsukas] No %s grammar facts found for %s.", lang, guid), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("[barsukas] Grammar facts for %s in %s:", guid, lang))
	for _, f := range facts {
		notes := ""
		if f.Notes != "" {
			notes = fmt.Sprintf(" (%s)", f.Notes)
		}
		sb.WriteString(fmt.Sprintf("\n  %s: %s%s", f.FactType, f.FactValue, notes))
	}
	return sb.String(), nil
}

func (b *BarsukasIntegration) handleSentences(ctx context.Context, lang, guid string) (string, error) {
	sentences, err := b.client.GetSentences(ctx, guid, lang)
	if err != nil {
		return "", err
	}
	if len(sentences) == 0 {
		return fmt.Sprintf("[barsukas] No sentences found for %s (lang: %s).", guid, lang), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("[barsukas] Example sentences for %s:", guid))
	shown := 0
	for _, s := range sentences {
		en := s.Translations["en"]
		target := s.Translations[lang]
		if en == "" && target == "" {
			continue
		}
		if en != "" && target != "" && lang != "en" {
			sb.WriteString(fmt.Sprintf("\n  %s / %s", en, target))
		} else if target != "" {
			sb.WriteString(fmt.Sprintf("\n  %s", target))
		} else {
			sb.WriteString(fmt.Sprintf("\n  %s", en))
		}
		shown++
		if shown >= 3 {
			break
		}
	}
	if shown == 0 {
		sb.WriteString("\n  No sentences with translations found.")
	}
	return sb.String(), nil
}

// formatResult formats a single SearchResult as the primary line.
func formatResult(r SearchResult) string {
	pos := r.PosType
	if r.PosSubtype != "" {
		pos = fmt.Sprintf("%s/%s", r.PosType, r.PosSubtype)
	}
	line := fmt.Sprintf("[barsukas] [%s] \"%s\" (%s) — %s", r.GUID, r.LemmaText, pos, r.Definition)
	if r.Disambiguation != "" {
		line += fmt.Sprintf(" (%s)", r.Disambiguation)
	}
	return line
}
