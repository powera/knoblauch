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

// BarsukasClient calls the Barsukas linguistics API.
type BarsukasClient struct {
	baseURL    string
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

// LemmaSummary is returned by the check_lemma_exists endpoint.
type LemmaSummary struct {
	ID             int    `json:"id"`
	LemmaText      string `json:"lemma_text"`
	PosType        string `json:"pos_type"`
	DefinitionText string `json:"definition_text"`
}

// CheckLemmaResult is the response from GET /api/check_lemma_exists.
type CheckLemmaResult struct {
	ExactMatch     *LemmaSummary  `json:"exact_match"`
	SimilarMatches []LemmaSummary `json:"similar_matches"`
}

// AutoPopulateResult is the response from GET /api/auto_populate_lemma.
type AutoPopulateResult struct {
	Definition          string `json:"definition"`
	PosType             string `json:"pos_type"`
	PosSubtype          string `json:"pos_subtype"`
	SuggestedDifficulty int    `json:"suggested_difficulty_level"`
}

// CheckLemmaExists queries for an exact and similar lemma matches.
// posType may be empty to search without a POS filter.
func (c *BarsukasClient) CheckLemmaExists(ctx context.Context, word, posType string) (*CheckLemmaResult, error) {
	params := url.Values{"search": {word}}
	if posType != "" {
		params.Set("pos_type", posType)
	}
	return doGet[CheckLemmaResult](ctx, c.httpClient, c.baseURL+"check_lemma_exists?"+params.Encode())
}

// AutoPopulateLemma uses the LLM-backed endpoint to generate definition and POS for a word.
func (c *BarsukasClient) AutoPopulateLemma(ctx context.Context, word string) (*AutoPopulateResult, error) {
	params := url.Values{"word": {word}}
	return doGet[AutoPopulateResult](ctx, c.httpClient, c.baseURL+"auto_populate_lemma?"+params.Encode())
}

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
