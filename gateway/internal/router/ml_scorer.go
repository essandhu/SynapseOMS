package router

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"time"

	"github.com/shopspring/decimal"
	"github.com/synapse-oms/gateway/internal/domain"
)

const defaultMLTimeout = 100 * time.Millisecond

// VenueScore holds a venue's ML-predicted score and rank.
type VenueScore struct {
	VenueID string  `json:"venue_id"`
	Score   float64 `json:"score"`
	Rank    int     `json:"rank"`
}

// MLScorerOption configures an MLScorer.
type MLScorerOption func(*MLScorer)

// WithMLTimeout sets the HTTP client timeout for calls to the ML sidecar.
func WithMLTimeout(d time.Duration) MLScorerOption {
	return func(s *MLScorer) {
		s.client.Timeout = d
	}
}

// WithHTTPClient replaces the default HTTP client entirely.
func WithHTTPClient(c *http.Client) MLScorerOption {
	return func(s *MLScorer) {
		s.client = c
	}
}

// MLScorer calls the ML sidecar's /score endpoint to rank venue candidates.
type MLScorer struct {
	client   *http.Client
	endpoint string // e.g. "http://localhost:8090/score"
}

// NewMLScorer creates an MLScorer that calls the given endpoint.
func NewMLScorer(endpoint string, opts ...MLScorerOption) *MLScorer {
	s := &MLScorer{
		client: &http.Client{
			Timeout: defaultMLTimeout,
		},
		endpoint: endpoint,
	}
	for _, o := range opts {
		o(s)
	}
	return s
}

// --- JSON request/response types matching the sidecar protocol ---

type scoreRequest struct {
	Order      scoreOrderFeatures      `json:"order"`
	Candidates []scoreCandidateFeature `json:"candidates"`
}

type scoreOrderFeatures struct {
	InstrumentID    string  `json:"instrument_id"`
	Side            string  `json:"side"`
	Quantity        string  `json:"quantity"`
	OrderSizePctADV float64 `json:"order_size_pct_adv"`
}

type scoreCandidateFeature struct {
	VenueID          string  `json:"venue_id"`
	SpreadBps        float64 `json:"spread_bps"`
	BookDepthAtPrice float64 `json:"book_depth_at_price"`
	LatencyMs        float64 `json:"latency_ms"`
	FillRate30d      float64 `json:"fill_rate_30d"`
	FeeRateBps       float64 `json:"fee_rate_bps"`
}

type scoreResponse struct {
	Scores []VenueScore `json:"scores"`
}

var (
	mlDecTwo  = decimal.NewFromInt(2)
	mlDecTenK = decimal.NewFromInt(10000)
)

// ScoreVenues sends candidate venues and order features to the ML sidecar
// and returns scored venues sorted by rank.
func (s *MLScorer) ScoreVenues(ctx context.Context, order *domain.Order, candidates []VenueCandidate) ([]VenueScore, error) {
	reqBody := buildScoreRequest(order, candidates)

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("ml_scorer: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, s.endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("ml_scorer: create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("ml_scorer: http post: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ml_scorer: sidecar returned status %d: %s", resp.StatusCode, string(body))
	}

	var result scoreResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("ml_scorer: decode response: %w", err)
	}

	// Sort by rank ascending.
	sort.Slice(result.Scores, func(i, j int) bool {
		return result.Scores[i].Rank < result.Scores[j].Rank
	})

	return result.Scores, nil
}

// buildScoreRequest converts domain types to the JSON structure the sidecar expects.
func buildScoreRequest(order *domain.Order, candidates []VenueCandidate) scoreRequest {
	side := "buy"
	if order.Side == domain.SideSell {
		side = "sell"
	}

	req := scoreRequest{
		Order: scoreOrderFeatures{
			InstrumentID:    order.InstrumentID,
			Side:            side,
			Quantity:        order.Quantity.String(),
			OrderSizePctADV: 0, // not available at this layer; sidecar can default
		},
		Candidates: make([]scoreCandidateFeature, len(candidates)),
	}

	for i, c := range candidates {
		// Spread in bps: ((ask - bid) / mid) * 10000
		var spreadBps float64
		mid := c.BidPrice.Add(c.AskPrice).Div(mlDecTwo)
		if mid.IsPositive() {
			spreadBps, _ = c.AskPrice.Sub(c.BidPrice).Div(mid).Mul(mlDecTenK).Float64()
		}

		depthAtPrice, _ := c.DepthAtPrice.Float64()
		feeRateBps, _ := c.FeeRate.Mul(mlDecTenK).Float64()

		req.Candidates[i] = scoreCandidateFeature{
			VenueID:          c.VenueID,
			SpreadBps:        spreadBps,
			BookDepthAtPrice: depthAtPrice,
			LatencyMs:        float64(c.LatencyP50.Milliseconds()),
			FillRate30d:      c.FillRate30d,
			FeeRateBps:       feeRateBps,
		}
	}

	return req
}
