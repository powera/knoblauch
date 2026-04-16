package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// SupportedLanguages lists the primary language codes the Barsukas API is built around.
// The server has data for ~50 more languages beyond these, but these are the ones
// with the most complete translation/audio/forms coverage.
var SupportedLanguages = []string{"en", "es", "fr", "lt", "zh", "pt", "it", "nl", "sv", "de", "ja"}

// IsValidLanguage reports whether code is a supported Barsukas language code.
func IsValidLanguage(code string) bool {
	for _, l := range SupportedLanguages {
		if l == code {
			return true
		}
	}
	return false
}

// BarsukasClient calls the Barsukas linguistics API (v1).
type BarsukasClient struct {
	baseURL    string // must end with /api/v1/
	httpClient *http.Client
}

func NewBarsukasClient(baseURL string) *BarsukasClient {
	return &BarsukasClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// --- Search ---

// SearchResult is one item from GET /api/v1/search.
type SearchResult struct {
	GUID         string            `json:"guid"`
	LemmaText    string            `json:"lemma_text"`
	Definition   string            `json:"definition"`
	PosType      string            `json:"pos_type"`
	PosSubtype   string            `json:"pos_subtype"`
	Difficulty   *int              `json:"difficulty_level"`
	Disambiguation string          `json:"disambiguation"`
	Translations map[string]string `json:"translations"`
	Verified     bool              `json:"verified"`
}

type searchResponse struct {
	Data []SearchResult `json:"data"`
}

// Search queries GET /api/v1/search.
// posType may be empty. limit=0 uses the server default.
func (c *BarsukasClient) Search(ctx context.Context, query, posType string, limit int) ([]SearchResult, error) {
	params := url.Values{"q": {query}}
	if posType != "" {
		params.Set("pos_type", posType)
	}
	if limit > 0 {
		params.Set("limit", fmt.Sprint(limit))
	}
	res, err := doGet[searchResponse](ctx, c.httpClient, c.baseURL+"search?"+params.Encode())
	if err != nil {
		return nil, err
	}
	return res.Data, nil
}

// --- Lemma detail ---

// LemmaDetail is returned by GET /api/v1/lemma/<guid>.
type LemmaDetail struct {
	GUID           string   `json:"guid"`
	LemmaText      string   `json:"lemma_text"`
	Definition     string   `json:"definition"`
	PosType        string   `json:"pos_type"`
	PosSubtype     string   `json:"pos_subtype"`
	Difficulty     *int     `json:"difficulty_level"`
	Verified       bool     `json:"verified"`
	Tags           []string `json:"tags"`
	Disambiguation string   `json:"disambiguation"`
}

type lemmaDetailResponse struct {
	Data LemmaDetail `json:"data"`
}

// GetLemma fetches GET /api/v1/lemma/<guid>.
func (c *BarsukasClient) GetLemma(ctx context.Context, guid string) (*LemmaDetail, error) {
	res, err := doGet[lemmaDetailResponse](ctx, c.httpClient, c.baseURL+"lemma/"+url.PathEscape(guid))
	if err != nil {
		return nil, err
	}
	return &res.Data, nil
}

// --- Translations ---

type translationsResponse struct {
	Data     map[string]string  `json:"data"`
	Metadata translationsMeta   `json:"metadata"`
}

type translationsMeta struct {
	GUID               string   `json:"guid"`
	AvailableLanguages []string `json:"available_languages"`
	IsPopulated        *bool    `json:"is_populated"`
}

// GetTranslations fetches GET /api/v1/lemma/<guid>/translations.
// langCode may be empty to get all languages.
func (c *BarsukasClient) GetTranslations(ctx context.Context, guid, langCode string) (map[string]string, []string, error) {
	u := c.baseURL + "lemma/" + url.PathEscape(guid) + "/translations"
	if langCode != "" {
		u += "?language=" + url.QueryEscape(langCode)
	}
	res, err := doGet[translationsResponse](ctx, c.httpClient, u)
	if err != nil {
		return nil, nil, err
	}
	return res.Data, res.Metadata.AvailableLanguages, nil
}

// --- Forms ---

// LemmaForm is one entry from GET /api/v1/lemma/<guid>/forms.
type LemmaForm struct {
	FormText              string `json:"form_text"`
	LanguageCode          string `json:"language_code"`
	GrammaticalForm       string `json:"grammatical_form"`
	IsBaseForm            bool   `json:"is_base_form"`
	IPAPronunciation      string `json:"ipa_pronunciation"`
	PhoneticPronunciation string `json:"phonetic_pronunciation"`
	Verified              bool   `json:"verified"`
}

type formsResponse struct {
	Data []LemmaForm `json:"data"`
}

// GetForms fetches GET /api/v1/lemma/<guid>/forms.
// langCode may be empty to get all languages.
func (c *BarsukasClient) GetForms(ctx context.Context, guid, langCode string) ([]LemmaForm, error) {
	u := c.baseURL + "lemma/" + url.PathEscape(guid) + "/forms"
	if langCode != "" {
		u += "?language=" + url.QueryEscape(langCode)
	}
	res, err := doGet[formsResponse](ctx, c.httpClient, u)
	if err != nil {
		return nil, err
	}
	return res.Data, nil
}

// --- Grammar ---

// GrammarFact is one entry from GET /api/v1/lemma/<guid>/grammar.
type GrammarFact struct {
	LanguageCode string `json:"language_code"`
	FactType     string `json:"fact_type"`
	FactValue    string `json:"fact_value"`
	Notes        string `json:"notes"`
	Verified     bool   `json:"verified"`
}

type grammarResponse struct {
	Data []GrammarFact `json:"data"`
}

// GetGrammar fetches GET /api/v1/lemma/<guid>/grammar.
// langCode may be empty to get all languages.
func (c *BarsukasClient) GetGrammar(ctx context.Context, guid, langCode string) ([]GrammarFact, error) {
	u := c.baseURL + "lemma/" + url.PathEscape(guid) + "/grammar"
	if langCode != "" {
		u += "?language=" + url.QueryEscape(langCode)
	}
	res, err := doGet[grammarResponse](ctx, c.httpClient, u)
	if err != nil {
		return nil, err
	}
	return res.Data, nil
}

// --- Sentences ---

// ExampleSentence is one entry from GET /api/v1/lemma/<guid>/sentences.
type ExampleSentence struct {
	SentenceID   int               `json:"sentence_id"`
	Translations map[string]string `json:"translations"`
	MinLevel     *int              `json:"minimum_level"`
	PatternType  string            `json:"pattern_type"`
	Tense        string            `json:"tense"`
	Verified     bool              `json:"verified"`
}

type sentencesResponse struct {
	Data []ExampleSentence `json:"data"`
}

// GetSentences fetches GET /api/v1/lemma/<guid>/sentences.
// langCode may be empty to get all languages.
func (c *BarsukasClient) GetSentences(ctx context.Context, guid, langCode string) ([]ExampleSentence, error) {
	u := c.baseURL + "lemma/" + url.PathEscape(guid) + "/sentences"
	if langCode != "" {
		u += "?language=" + url.QueryEscape(langCode)
	}
	res, err := doGet[sentencesResponse](ctx, c.httpClient, u)
	if err != nil {
		return nil, err
	}
	return res.Data, nil
}

// --- Pronunciations ---

// Pronunciation is the value in the map returned by GET /api/v1/lemma/<guid>/pronunciations.
type Pronunciation struct {
	IPA      string `json:"ipa"`
	Phonetic string `json:"phonetic"`
}

type pronunciationsResponse struct {
	Data     map[string]Pronunciation `json:"data"`
	Metadata pronunciationsMeta       `json:"metadata"`
}

type pronunciationsMeta struct {
	GUID                       string   `json:"guid"`
	LanguagesWithPronunciations []string `json:"languages_with_pronunciations"`
}

// GetPronunciations fetches GET /api/v1/lemma/<guid>/pronunciations.
// langCode may be empty to get all languages.
// Returns a map of language code to pronunciation and the sorted list of languages that have one.
func (c *BarsukasClient) GetPronunciations(ctx context.Context, guid, langCode string) (map[string]Pronunciation, []string, error) {
	u := c.baseURL + "lemma/" + url.PathEscape(guid) + "/pronunciations"
	if langCode != "" {
		u += "?language=" + url.QueryEscape(langCode)
	}
	res, err := doGet[pronunciationsResponse](ctx, c.httpClient, u)
	if err != nil {
		return nil, nil, err
	}
	return res.Data, res.Metadata.LanguagesWithPronunciations, nil
}

// --- Audio (per-lemma) ---

// LemmaAudio is the value for each language in GET /api/v1/lemma/<guid>/audio.
type LemmaAudio struct {
	HasLemmaAudio  bool `json:"has_lemma_audio"`
	FormAudioCount int  `json:"form_audio_count"`
}

type lemmaAudioResponse struct {
	Data     map[string]LemmaAudio `json:"data"`
	Metadata lemmaAudioMeta        `json:"metadata"`
}

type lemmaAudioMeta struct {
	GUID                    string   `json:"guid"`
	LanguagesWithLemmaAudio []string `json:"languages_with_lemma_audio"`
	LanguagesWithAnyAudio   []string `json:"languages_with_any_audio"`
}

// GetAudio fetches GET /api/v1/lemma/<guid>/audio.
// langCode may be empty for all languages. Returns the per-language data and
// two metadata lists: languages with a lemma-level recording, and languages
// with any audio (lemma- or form-level).
func (c *BarsukasClient) GetAudio(ctx context.Context, guid, langCode string) (map[string]LemmaAudio, []string, []string, error) {
	u := c.baseURL + "lemma/" + url.PathEscape(guid) + "/audio"
	if langCode != "" {
		u += "?language=" + url.QueryEscape(langCode)
	}
	res, err := doGet[lemmaAudioResponse](ctx, c.httpClient, u)
	if err != nil {
		return nil, nil, nil, err
	}
	return res.Data, res.Metadata.LanguagesWithLemmaAudio, res.Metadata.LanguagesWithAnyAudio, nil
}

// --- Word metadata (global stats) ---

// WordAudioCounts is the "audio" sub-object in a WordMetadata entry.
type WordAudioCounts struct {
	WithAudio    int `json:"with_audio"`
	WithoutAudio int `json:"without_audio"`
}

// WordDerivativeCounts is the "derivative_forms" sub-object in a WordMetadata entry.
type WordDerivativeCounts struct {
	WithDerivativeForms    int `json:"with_derivative_forms"`
	WithoutDerivativeForms int `json:"without_derivative_forms"`
}

// WordMetadata is one per-language entry from GET /api/v1/metadata/words.
type WordMetadata struct {
	TotalWords      int                  `json:"total_words"`
	WordsBySubtype  map[string]int       `json:"words_by_subtype"`
	Audio           WordAudioCounts      `json:"audio"`
	DerivativeForms WordDerivativeCounts `json:"derivative_forms"`
}

type wordMetadataResponse struct {
	Data     map[string]WordMetadata `json:"data"`
	Metadata wordMetadataMeta        `json:"metadata"`
}

type wordMetadataMeta struct {
	Languages []string `json:"languages"`
	Count     int      `json:"count"`
}

// GetWordMetadata fetches GET /api/v1/metadata/words.
// langCode may be empty to get all languages. maxDifficulty=0 means unset.
// Returns the per-language data map and the ordered language list from metadata.
func (c *BarsukasClient) GetWordMetadata(ctx context.Context, langCode string, maxDifficulty int) (map[string]WordMetadata, []string, error) {
	params := url.Values{}
	if langCode != "" {
		params.Set("language", langCode)
	}
	if maxDifficulty > 0 {
		params.Set("max_difficulty", fmt.Sprint(maxDifficulty))
	}
	u := c.baseURL + "metadata/words"
	if q := params.Encode(); q != "" {
		u += "?" + q
	}
	res, err := doGet[wordMetadataResponse](ctx, c.httpClient, u)
	if err != nil {
		return nil, nil, err
	}
	return res.Data, res.Metadata.Languages, nil
}

// --- helpers ---

func doGet[T any](ctx context.Context, client *http.Client, rawURL string) (*T, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("status %d: %s", resp.StatusCode, body)
	}
	var result T
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &result, nil
}
