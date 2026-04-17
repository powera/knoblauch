package integration

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strconv"
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
//	@barsukas status <guid>            — translation/pronunciation/sentence coverage for a GUID
//	@barsukas stats [lang]             — corpus stats (all languages, or one)
//	@barsukas progress words <lang> [max_level]
//	@barsukas progress sentences <lang> [max_level]
//	@barsukas progress compare <words|sentences> <lang1> <lang2> [max_level]
//	@barsukas audio <lang> <guid> [form] — inline audio player (lemma-level by default)
//	@barsukas help                     — list all commands
//
// Language codes: en zh fr lt ko es de pt sw vi
type BarsukasIntegration struct {
	client *BarsukasClient
	secret []byte // used to HMAC-sign audio-proxy tokens
}

func NewBarsukasIntegration(baseURL string, secret []byte) *BarsukasIntegration {
	return &BarsukasIntegration{client: NewBarsukasClient(baseURL), secret: secret}
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
	"  @barsukas status <guid>            coverage summary for a lemma\n" +
	"  @barsukas stats [lang]             corpus stats (all languages, or one)\n" +
	"  @barsukas progress words <lang> [max_level]                 word coverage metrics\n" +
	"  @barsukas progress sentences <lang> [max_level]             sentence coverage metrics\n" +
	"  @barsukas progress compare <words|sentences> <lang1> <lang2> [max_level]\n" +
	"                                         compare coverage side-by-side\n" +
	"  @barsukas audio <lang> <guid> [form]  inline audio player\n" +
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
	case "status":
		if rest == "" {
			return "[barsukas] Usage: @barsukas status <guid>", nil
		}
		return b.handleStatus(ctx, rest)
	case "stats":
		return b.handleStats(ctx, rest)
	case "progress":
		return b.handleProgress(ctx, rest)
	case "sentences":
		lang, guid, ok := splitLangArg(rest)
		if !ok {
			return "[barsukas] Usage: @barsukas sentences <lang> <guid>  (e.g. sentences lt V01_001)", nil
		}
		if msg := checkLang(lang); msg != "" {
			return msg, nil
		}
		return b.handleSentences(ctx, lang, guid)
	case "audio":
		lang, tail, ok := splitLangArg(rest)
		if !ok {
			return "[barsukas] Usage: @barsukas audio <lang> <guid> [form]  (e.g. audio lt V01_001)", nil
		}
		if msg := checkLang(lang); msg != "" {
			return msg, nil
		}
		parts := strings.SplitN(tail, " ", 2)
		guid := parts[0]
		form := ""
		if len(parts) == 2 {
			form = strings.TrimSpace(parts[1])
		}
		return b.handleAudio(ctx, lang, guid, form)
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

func (b *BarsukasIntegration) handleStatus(ctx context.Context, guid string) (string, error) {
	lemma, err := b.client.GetLemma(ctx, guid)
	if err != nil {
		return fmt.Sprintf("[barsukas] Could not fetch %s: %v", guid, err), nil
	}

	translations, translationLangs, err := b.client.GetTranslations(ctx, guid, "")
	if err != nil {
		return "", err
	}
	// Keep only languages that have a non-empty translation.
	var presentTranslations []string
	for lang, t := range translations {
		if strings.TrimSpace(t) != "" {
			presentTranslations = append(presentTranslations, lang)
		}
	}
	sort.Strings(presentTranslations)

	_, pronLangs, err := b.client.GetPronunciations(ctx, guid, "")
	if err != nil {
		return "", err
	}
	sort.Strings(pronLangs)

	sentences, err := b.client.GetSentences(ctx, guid, "")
	if err != nil {
		return "", err
	}
	sentenceLangSet := map[string]bool{}
	for _, s := range sentences {
		for lang, t := range s.Translations {
			if strings.TrimSpace(t) != "" {
				sentenceLangSet[lang] = true
			}
		}
	}
	var sentenceLangs []string
	for l := range sentenceLangSet {
		sentenceLangs = append(sentenceLangs, l)
	}
	sort.Strings(sentenceLangs)

	forms, err := b.client.GetForms(ctx, guid, "")
	if err != nil {
		return "", err
	}
	formLangSet := map[string]bool{}
	inflectedLangSet := map[string]bool{}
	for _, f := range forms {
		formLangSet[f.LanguageCode] = true
		if !f.IsBaseForm {
			inflectedLangSet[f.LanguageCode] = true
		}
	}
	formLangs := sortedKeys(formLangSet)

	audioData, lemmaAudioLangs, _, err := b.client.GetAudio(ctx, guid, "")
	if err != nil {
		return "", err
	}
	sort.Strings(lemmaAudioLangs)
	audioSet := toSet(lemmaAudioLangs)

	grammar, err := b.client.GetGrammar(ctx, guid, "")
	if err != nil {
		return "", err
	}
	grammarLangSet := map[string]bool{}
	for _, g := range grammar {
		grammarLangSet[g.LanguageCode] = true
	}
	grammarLangs := sortedKeys(grammarLangSet)

	// If translations metadata gave us a list, prefer it (covers cases where the
	// map was filtered server-side but metadata still reports the set).
	if len(presentTranslations) == 0 && len(translationLangs) > 0 {
		presentTranslations = append(presentTranslations, translationLangs...)
		sort.Strings(presentTranslations)
	}

	pos := lemma.PosType
	if lemma.PosSubtype != "" {
		pos = fmt.Sprintf("%s/%s", lemma.PosType, lemma.PosSubtype)
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "[barsukas] Status for %s \"%s\" (%s):", lemma.GUID, lemma.LemmaText, pos)
	sb.WriteString(statusLine("Translations", presentTranslations))
	sb.WriteString(statusLine("Pronunciations", pronLangs))
	sb.WriteString(audioLine(audioData, lemmaAudioLangs))
	sb.WriteString(formsLine(formLangs, inflectedLangSet))
	sb.WriteString(statusLine("Grammar facts", grammarLangs))
	if len(sentences) == 0 {
		sb.WriteString("\n  Sentences:     none")
	} else {
		fmt.Fprintf(&sb, "\n  Sentences:     %d total", len(sentences))
		if len(sentenceLangs) > 0 {
			fmt.Fprintf(&sb, ", with translations in: %s", strings.Join(sentenceLangs, ", "))
		}
	}

	// Audit: what's missing among the primary supported languages?
	translationSet := toSet(presentTranslations)
	pronSet := toSet(pronLangs)
	sentenceSetCopy := sentenceLangSet
	missingTranslation := missingFromPrimary(translationSet)
	missingPron := missingFromPrimary(pronSet)
	missingAudio := missingFromPrimary(audioSet)
	missingForms := missingFromPrimary(formLangSet)
	missingSentences := missingFromPrimary(sentenceSetCopy)
	missingGrammar := missingFromPrimary(grammarLangSet)
	if len(missingTranslation)+len(missingPron)+len(missingAudio)+len(missingForms)+len(missingSentences)+len(missingGrammar) > 0 {
		sb.WriteString("\n  Missing (primary languages only):")
		if len(missingTranslation) > 0 {
			fmt.Fprintf(&sb, "\n    translations: %s", strings.Join(missingTranslation, ", "))
		}
		if len(missingPron) > 0 {
			fmt.Fprintf(&sb, "\n    pronunciations: %s", strings.Join(missingPron, ", "))
		}
		if len(missingAudio) > 0 {
			fmt.Fprintf(&sb, "\n    audio: %s", strings.Join(missingAudio, ", "))
		}
		if len(missingForms) > 0 {
			fmt.Fprintf(&sb, "\n    forms: %s", strings.Join(missingForms, ", "))
		}
		if len(missingSentences) > 0 {
			fmt.Fprintf(&sb, "\n    sentences: %s", strings.Join(missingSentences, ", "))
		}
		if len(missingGrammar) > 0 {
			fmt.Fprintf(&sb, "\n    grammar facts: %s", strings.Join(missingGrammar, ", "))
		}
	}
	return sb.String(), nil
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func toSet(xs []string) map[string]bool {
	s := make(map[string]bool, len(xs))
	for _, x := range xs {
		s[x] = true
	}
	return s
}

// missingFromPrimary returns the SupportedLanguages codes absent from have, in
// SupportedLanguages order. English is skipped for "translations" / "forms" etc.
// where absence is meaningless — caller filters as needed; this helper just
// reports raw absences.
func missingFromPrimary(have map[string]bool) []string {
	var missing []string
	for _, l := range SupportedLanguages {
		if !have[l] {
			missing = append(missing, l)
		}
	}
	return missing
}

func audioLine(data map[string]LemmaAudio, lemmaLangs []string) string {
	if len(data) == 0 {
		return "\n  Audio: none"
	}
	totalFormAudio := 0
	for _, a := range data {
		totalFormAudio += a.FormAudioCount
	}
	if len(lemmaLangs) == 0 {
		// Only form-level recordings exist, no lemma-level.
		return fmt.Sprintf("\n  Audio: none at lemma level [+%d form recordings]", totalFormAudio)
	}
	if totalFormAudio == 0 {
		return fmt.Sprintf("\n  Audio (%d): %s", len(lemmaLangs), strings.Join(lemmaLangs, ", "))
	}
	return fmt.Sprintf("\n  Audio (%d): %s [+%d form recordings]",
		len(lemmaLangs), strings.Join(lemmaLangs, ", "), totalFormAudio)
}

func formsLine(langs []string, inflected map[string]bool) string {
	if len(langs) == 0 {
		return "\n  Forms: none"
	}
	inflectedCount := len(inflected)
	baseOnly := len(langs) - inflectedCount
	return fmt.Sprintf("\n  Forms (%d): %s [%d with inflections, %d base only]",
		len(langs), strings.Join(langs, ", "), inflectedCount, baseOnly)
}

func statusLine(label string, langs []string) string {
	if len(langs) == 0 {
		return fmt.Sprintf("\n  %s: none", label)
	}
	return fmt.Sprintf("\n  %s (%d): %s", label, len(langs), strings.Join(langs, ", "))
}

func (b *BarsukasIntegration) handleStats(ctx context.Context, rest string) (string, error) {
	lang := strings.ToLower(strings.TrimSpace(rest))
	// Deliberately do NOT validate against SupportedLanguages — the server
	// has data for many languages beyond our "fully supported" set.

	data, order, err := b.client.GetWordMetadata(ctx, lang, 0)
	if err != nil {
		return fmt.Sprintf("[barsukas] Stats fetch failed: %v", err), nil
	}
	if len(data) == 0 {
		return "[barsukas] No stats available.", nil
	}

	// Base corpus size from English.
	baseLine := ""
	if en, ok := data["en"]; ok {
		baseLine = fmt.Sprintf(" (English base: %d lemmas)", en.TotalWords)
	}

	// Order: prefer metadata.languages if present, otherwise sorted keys.
	languages := order
	if len(languages) == 0 {
		for l := range data {
			languages = append(languages, l)
		}
		sort.Strings(languages)
	}

	var sb strings.Builder
	if lang != "" {
		fmt.Fprintf(&sb, "[barsukas] Stats for %s%s:", lang, baseLine)
	} else {
		fmt.Fprintf(&sb, "[barsukas] Corpus stats%s:", baseLine)
	}
	for _, l := range languages {
		m, ok := data[l]
		if !ok {
			continue
		}
		fmt.Fprintf(&sb, "\n  %s: %d words, %d with audio, %d with derivative forms",
			l, m.TotalWords, m.Audio.WithAudio, m.DerivativeForms.WithDerivativeForms)
	}
	return sb.String(), nil
}

func (b *BarsukasIntegration) handleProgress(ctx context.Context, rest string) (string, error) {
	parts := strings.Fields(strings.TrimSpace(rest))
	if len(parts) == 0 {
		return "[barsukas] Usage: @barsukas progress <words|sentences|compare> ...", nil
	}

	switch strings.ToLower(parts[0]) {
	case "words":
		if len(parts) < 2 {
			return "[barsukas] Usage: @barsukas progress words <language> [max_level]", nil
		}
		lang := strings.ToLower(parts[1])
		maxLevel, errMsg := parseMaxLevel(parts[2:])
		if errMsg != "" {
			return errMsg, nil
		}
		data, _, err := b.client.GetWordMetadata(ctx, lang, maxLevel)
		if err != nil {
			return fmt.Sprintf("[barsukas] Progress fetch failed: %v", err), nil
		}
		m, ok := data[lang]
		if !ok {
			return fmt.Sprintf("[barsukas] No word metadata for %s.", lang), nil
		}
		return formatWordProgress(lang, maxLevel, m), nil
	case "sentences":
		if len(parts) < 2 {
			return "[barsukas] Usage: @barsukas progress sentences <language> [max_level]", nil
		}
		lang := strings.ToLower(parts[1])
		maxLevel, errMsg := parseMaxLevel(parts[2:])
		if errMsg != "" {
			return errMsg, nil
		}
		data, _, err := b.client.GetSentenceMetadata(ctx, lang, maxLevel)
		if err != nil {
			return fmt.Sprintf("[barsukas] Progress fetch failed: %v", err), nil
		}
		m, ok := data[lang]
		if !ok {
			return fmt.Sprintf("[barsukas] No sentence metadata for %s.", lang), nil
		}
		return formatSentenceProgress(lang, maxLevel, m), nil
	case "compare":
		if len(parts) < 4 {
			return "[barsukas] Usage: @barsukas progress compare <words|sentences> <lang1> <lang2> [max_level]", nil
		}
		kind := strings.ToLower(parts[1])
		lang1 := strings.ToLower(parts[2])
		lang2 := strings.ToLower(parts[3])
		maxLevel, errMsg := parseMaxLevel(parts[4:])
		if errMsg != "" {
			return errMsg, nil
		}
		if kind == "words" {
			return b.compareWordProgress(ctx, lang1, lang2, maxLevel)
		}
		if kind == "sentences" {
			return b.compareSentenceProgress(ctx, lang1, lang2, maxLevel)
		}
		return "[barsukas] Usage: @barsukas progress compare <words|sentences> <lang1> <lang2> [max_level]", nil
	default:
		return "[barsukas] Usage: @barsukas progress <words|sentences|compare> ...", nil
	}
}

func parseMaxLevel(parts []string) (int, string) {
	if len(parts) == 0 {
		return 0, ""
	}
	if len(parts) > 1 {
		return 0, "[barsukas] max_level must be a single integer in range 1..9."
	}
	n, err := strconv.Atoi(parts[0])
	if err != nil || n < 1 || n > 9 {
		return 0, "[barsukas] max_level must be an integer in range 1..9."
	}
	return n, ""
}

func formatWordProgress(lang string, maxLevel int, m WordMetadata) string {
	scope := "all levels"
	if maxLevel > 0 {
		scope = fmt.Sprintf("max_level=%d", maxLevel)
	}
	audioPct := percent(m.Audio.WithAudio, m.TotalWords)
	derivPct := percent(m.DerivativeForms.WithDerivativeForms, m.TotalWords)
	return fmt.Sprintf("[barsukas] Word progress (%s, %s):\n  total words: %d\n  audio coverage: %s (%d/%d)\n  derivative-form coverage: %s (%d/%d)",
		lang, scope, m.TotalWords,
		audioPct, m.Audio.WithAudio, m.TotalWords,
		derivPct, m.DerivativeForms.WithDerivativeForms, m.TotalWords)
}

func formatSentenceProgress(lang string, maxLevel int, m SentenceMetadata) string {
	scope := "all levels"
	if maxLevel > 0 {
		scope = fmt.Sprintf("max_level=%d", maxLevel)
	}
	audioPct := percent(m.Audio.WithAudio, m.TotalSentences)
	verifiedPct := percent(m.Verified.WithVerified, m.TotalSentences)
	return fmt.Sprintf("[barsukas] Sentence progress (%s, %s):\n  total sentences: %d\n  audio coverage: %s (%d/%d)\n  verified coverage: %s (%d/%d)",
		lang, scope, m.TotalSentences,
		audioPct, m.Audio.WithAudio, m.TotalSentences,
		verifiedPct, m.Verified.WithVerified, m.TotalSentences)
}

func (b *BarsukasIntegration) compareWordProgress(ctx context.Context, lang1, lang2 string, maxLevel int) (string, error) {
	data1, _, err := b.client.GetWordMetadata(ctx, lang1, maxLevel)
	if err != nil {
		return fmt.Sprintf("[barsukas] Progress fetch failed for %s: %v", lang1, err), nil
	}
	data2, _, err := b.client.GetWordMetadata(ctx, lang2, maxLevel)
	if err != nil {
		return fmt.Sprintf("[barsukas] Progress fetch failed for %s: %v", lang2, err), nil
	}
	m1, ok1 := data1[lang1]
	m2, ok2 := data2[lang2]
	if !ok1 || !ok2 {
		return fmt.Sprintf("[barsukas] Missing word metadata for %s or %s.", lang1, lang2), nil
	}

	scope := "all levels"
	if maxLevel > 0 {
		scope = fmt.Sprintf("max_level=%d", maxLevel)
	}
	audio1 := percentFloat(m1.Audio.WithAudio, m1.TotalWords)
	audio2 := percentFloat(m2.Audio.WithAudio, m2.TotalWords)
	deriv1 := percentFloat(m1.DerivativeForms.WithDerivativeForms, m1.TotalWords)
	deriv2 := percentFloat(m2.DerivativeForms.WithDerivativeForms, m2.TotalWords)

	var sb strings.Builder
	fmt.Fprintf(&sb, "[barsukas] Word progress comparison (%s vs %s, %s):", lang1, lang2, scope)
	fmt.Fprintf(&sb, "\n  metric                    %-15s %-15s", strings.ToUpper(lang1), strings.ToUpper(lang2))
	fmt.Fprintf(&sb, "\n  total words               %-15d %-15d", m1.TotalWords, m2.TotalWords)
	fmt.Fprintf(&sb, "\n  audio coverage            %-15s %-15s", percentStr(audio1), percentStr(audio2))
	fmt.Fprintf(&sb, "\n  derivative-form coverage  %-15s %-15s", percentStr(deriv1), percentStr(deriv2))
	appendGapLine(&sb, lang1, lang2, "audio coverage", audio1, audio2)
	appendGapLine(&sb, lang1, lang2, "derivative-form coverage", deriv1, deriv2)
	return sb.String(), nil
}

func (b *BarsukasIntegration) compareSentenceProgress(ctx context.Context, lang1, lang2 string, maxLevel int) (string, error) {
	data1, _, err := b.client.GetSentenceMetadata(ctx, lang1, maxLevel)
	if err != nil {
		return fmt.Sprintf("[barsukas] Progress fetch failed for %s: %v", lang1, err), nil
	}
	data2, _, err := b.client.GetSentenceMetadata(ctx, lang2, maxLevel)
	if err != nil {
		return fmt.Sprintf("[barsukas] Progress fetch failed for %s: %v", lang2, err), nil
	}
	m1, ok1 := data1[lang1]
	m2, ok2 := data2[lang2]
	if !ok1 || !ok2 {
		return fmt.Sprintf("[barsukas] Missing sentence metadata for %s or %s.", lang1, lang2), nil
	}

	scope := "all levels"
	if maxLevel > 0 {
		scope = fmt.Sprintf("max_level=%d", maxLevel)
	}
	audio1 := percentFloat(m1.Audio.WithAudio, m1.TotalSentences)
	audio2 := percentFloat(m2.Audio.WithAudio, m2.TotalSentences)
	verified1 := percentFloat(m1.Verified.WithVerified, m1.TotalSentences)
	verified2 := percentFloat(m2.Verified.WithVerified, m2.TotalSentences)

	var sb strings.Builder
	fmt.Fprintf(&sb, "[barsukas] Sentence progress comparison (%s vs %s, %s):", lang1, lang2, scope)
	fmt.Fprintf(&sb, "\n  metric                    %-15s %-15s", strings.ToUpper(lang1), strings.ToUpper(lang2))
	fmt.Fprintf(&sb, "\n  total sentences           %-15d %-15d", m1.TotalSentences, m2.TotalSentences)
	fmt.Fprintf(&sb, "\n  audio coverage            %-15s %-15s", percentStr(audio1), percentStr(audio2))
	fmt.Fprintf(&sb, "\n  verified coverage         %-15s %-15s", percentStr(verified1), percentStr(verified2))
	appendGapLine(&sb, lang1, lang2, "audio coverage", audio1, audio2)
	appendGapLine(&sb, lang1, lang2, "verified coverage", verified1, verified2)
	return sb.String(), nil
}

func appendGapLine(sb *strings.Builder, lang1, lang2, metric string, a, b float64) {
	if math.Abs(a-b) < 0.05 {
		fmt.Fprintf(sb, "\n  gap: %s and %s are essentially tied on %s.", strings.ToUpper(lang1), strings.ToUpper(lang2), metric)
		return
	}
	if a < b {
		fmt.Fprintf(sb, "\n  gap: %s %s trails %s by %.1f%%.", strings.ToUpper(lang1), metric, strings.ToUpper(lang2), b-a)
		return
	}
	fmt.Fprintf(sb, "\n  gap: %s %s trails %s by %.1f%%.", strings.ToUpper(lang2), metric, strings.ToUpper(lang1), a-b)
}

func percent(with, total int) string {
	return percentStr(percentFloat(with, total))
}

func percentFloat(with, total int) float64 {
	if total <= 0 {
		return 0
	}
	return float64(with) * 100 / float64(total)
}

func percentStr(v float64) string {
	return fmt.Sprintf("%.1f%%", v)
}

// handleAudio picks one audio file for the given lemma/language and emits a
// message containing a signed ::audio:: sentinel that the markup renderer
// turns into an <audio> tag.
func (b *BarsukasIntegration) handleAudio(ctx context.Context, lang, guid, form string) (string, error) {
	data, _, _, err := b.client.GetAudio(ctx, guid, lang)
	if err != nil {
		return "", err
	}
	entry, ok := data[lang]
	if !ok || len(entry.AudioFiles) == 0 {
		return fmt.Sprintf("[barsukas] No %s audio for %s.", lang, guid), nil
	}

	// Pick: matching form if the user asked, else the lemma-level entry
	// (grammatical_form == ""), else the first file.
	var chosen *AudioFile
	for i := range entry.AudioFiles {
		f := &entry.AudioFiles[i]
		if f.AudioURL == "" {
			continue
		}
		if form != "" {
			if strings.EqualFold(f.GrammaticalForm, form) {
				chosen = f
				break
			}
			continue
		}
		if f.GrammaticalForm == "" {
			chosen = f
			break
		}
	}
	if chosen == nil {
		for i := range entry.AudioFiles {
			if entry.AudioFiles[i].AudioURL != "" {
				chosen = &entry.AudioFiles[i]
				break
			}
		}
	}
	if chosen == nil {
		return fmt.Sprintf("[barsukas] No playable audio URL for %s (%s).", guid, lang), nil
	}

	if b.secret == nil {
		return "[barsukas] Audio proxy is not configured on this server.", nil
	}
	token, err := SignAudioURL(b.secret, chosen.AudioURL)
	if err != nil {
		return "", err
	}

	formLabel := chosen.GrammaticalForm
	if formLabel == "" {
		formLabel = "lemma"
	}
	voice := chosen.DisplayVoice
	if voice == "" {
		voice = chosen.VoiceName
	}
	header := fmt.Sprintf("[barsukas] Audio: %s (%s, %s, voice=%s)", guid, lang, formLabel, voice)
	return header + "\n::audio::" + token, nil
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
