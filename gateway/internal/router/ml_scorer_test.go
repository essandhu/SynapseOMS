package router

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/synapse-oms/gateway/internal/domain"
)

func newTestOrder() *domain.Order {
	return &domain.Order{
		ID:           "ord-ml-1",
		InstrumentID: "ETH-USD",
		Side:         domain.SideBuy,
		Quantity:     decimal.NewFromInt(10),
	}
}

func newTestCandidates() []VenueCandidate {
	return []VenueCandidate{
		{
			VenueID:      "binance",
			BidPrice:     decimal.NewFromFloat(1999),
			AskPrice:     decimal.NewFromFloat(2001),
			DepthAtPrice: decimal.NewFromInt(50),
			LatencyP50:   5 * time.Millisecond,
			FillRate30d:  0.95,
			FeeRate:      decimal.NewFromFloat(0.001),
		},
		{
			VenueID:      "sim",
			BidPrice:     decimal.NewFromFloat(1998),
			AskPrice:     decimal.NewFromFloat(2002),
			DepthAtPrice: decimal.NewFromInt(100),
			LatencyP50:   2 * time.Millisecond,
			FillRate30d:  0.90,
			FeeRate:      decimal.NewFromFloat(0.0005),
		},
	}
}

func TestMLScorer_ScoreVenues_ParsesResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected application/json content type")
		}

		var req scoreRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}

		if req.Order.InstrumentID != "ETH-USD" {
			t.Errorf("expected ETH-USD, got %s", req.Order.InstrumentID)
		}
		if len(req.Candidates) != 2 {
			t.Fatalf("expected 2 candidates, got %d", len(req.Candidates))
		}

		resp := scoreResponse{
			Scores: []VenueScore{
				{VenueID: "sim", Score: 0.92, Rank: 1},
				{VenueID: "binance", Score: 0.85, Rank: 2},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	scorer := NewMLScorer(srv.URL, WithMLTimeout(5*time.Second))
	scores, err := scorer.ScoreVenues(context.Background(), newTestOrder(), newTestCandidates())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(scores) != 2 {
		t.Fatalf("expected 2 scores, got %d", len(scores))
	}
	if scores[0].VenueID != "sim" {
		t.Errorf("expected rank 1 = sim, got %s", scores[0].VenueID)
	}
	if scores[0].Rank != 1 {
		t.Errorf("expected rank 1, got %d", scores[0].Rank)
	}
	if scores[1].VenueID != "binance" {
		t.Errorf("expected rank 2 = binance, got %s", scores[1].VenueID)
	}
}

func TestMLScorer_SidecarDown_ReturnsError(t *testing.T) {
	scorer := NewMLScorer("http://127.0.0.1:1", WithMLTimeout(50*time.Millisecond))
	_, err := scorer.ScoreVenues(context.Background(), newTestOrder(), newTestCandidates())
	if err == nil {
		t.Fatal("expected error when sidecar is unreachable")
	}
}

func TestMLScorer_SidecarReturns500(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	}))
	defer srv.Close()

	scorer := NewMLScorer(srv.URL, WithMLTimeout(5*time.Second))
	_, err := scorer.ScoreVenues(context.Background(), newTestOrder(), newTestCandidates())
	if err == nil {
		t.Fatal("expected error on 500 response")
	}
}

func TestMLStrategy_Name(t *testing.T) {
	scorer := NewMLScorer("http://localhost:8090/score")
	fallback := NewBestPriceStrategy()
	strategy := NewMLStrategy(scorer, fallback)

	if strategy.Name() != "ml-scored" {
		t.Errorf("expected 'ml-scored', got %s", strategy.Name())
	}
}

func TestMLStrategy_UsesMLScoresWhenAvailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		resp := scoreResponse{
			Scores: []VenueScore{
				{VenueID: "sim", Score: 0.92, Rank: 1},
				{VenueID: "binance", Score: 0.85, Rank: 2},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	scorer := NewMLScorer(srv.URL, WithMLTimeout(5*time.Second))
	fallback := NewBestPriceStrategy()
	strategy := NewMLStrategy(scorer, fallback)

	allocs, err := strategy.Evaluate(context.Background(), newTestOrder(), newTestCandidates())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// sim has 100 depth >= 10 order qty, so single allocation to sim
	if len(allocs) != 1 {
		t.Fatalf("expected 1 allocation, got %d", len(allocs))
	}
	if allocs[0].VenueID != "sim" {
		t.Errorf("expected sim (ML rank 1), got %s", allocs[0].VenueID)
	}
	if !allocs[0].Quantity.Equal(decimal.NewFromInt(10)) {
		t.Errorf("expected qty 10, got %s", allocs[0].Quantity)
	}
}

func TestMLStrategy_FallsBackOnSidecarError(t *testing.T) {
	// Point scorer at unreachable endpoint.
	scorer := NewMLScorer("http://127.0.0.1:1", WithMLTimeout(50*time.Millisecond))
	fallback := NewBestPriceStrategy()
	strategy := NewMLStrategy(scorer, fallback)

	allocs, err := strategy.Evaluate(context.Background(), newTestOrder(), newTestCandidates())
	if err != nil {
		t.Fatalf("expected fallback to succeed, got error: %v", err)
	}

	// Fallback to best-price: should return single allocation (cheapest ask has enough depth)
	if len(allocs) == 0 {
		t.Fatal("expected at least one allocation from fallback")
	}
	// Best ask is binance (2001) < sim (2002) for BUY
	if allocs[0].VenueID != "binance" {
		t.Errorf("expected fallback to pick binance (lowest ask), got %s", allocs[0].VenueID)
	}
}

func TestMLStrategy_EmptyCandidates_ReturnsError(t *testing.T) {
	scorer := NewMLScorer("http://localhost:8090/score")
	fallback := NewBestPriceStrategy()
	strategy := NewMLStrategy(scorer, fallback)

	_, err := strategy.Evaluate(context.Background(), newTestOrder(), nil)
	if err != ErrNoCandidatesForStrategy {
		t.Errorf("expected ErrNoCandidatesForStrategy, got %v", err)
	}
}

func TestMLStrategy_FallsBackOnEmptyScores(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		resp := scoreResponse{Scores: []VenueScore{}}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	scorer := NewMLScorer(srv.URL, WithMLTimeout(5*time.Second))
	fallback := NewBestPriceStrategy()
	strategy := NewMLStrategy(scorer, fallback)

	allocs, err := strategy.Evaluate(context.Background(), newTestOrder(), newTestCandidates())
	if err != nil {
		t.Fatalf("expected fallback, got error: %v", err)
	}
	if len(allocs) == 0 {
		t.Fatal("expected fallback allocations")
	}
}
