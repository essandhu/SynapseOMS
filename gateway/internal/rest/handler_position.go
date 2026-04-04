package rest

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/synapse-oms/gateway/internal/apperror"
	"github.com/synapse-oms/gateway/internal/domain"
)

// positionResponse is the JSON response for a position.
type positionResponse struct {
	InstrumentID      string    `json:"instrument_id"`
	VenueID           string    `json:"venue_id"`
	Quantity          string    `json:"quantity"`
	AverageCost       string    `json:"average_cost"`
	MarketPrice       string    `json:"market_price"`
	UnrealizedPnL     string    `json:"unrealized_pnl"`
	RealizedPnL       string    `json:"realized_pnl"`
	UnsettledQuantity string    `json:"unsettled_quantity"`
	SettledQuantity   string    `json:"settled_quantity"`
	AssetClass        string    `json:"asset_class"`
	QuoteCurrency     string    `json:"quote_currency"`
	UpdatedAt         time.Time `json:"updated_at"`
}

func toPositionResponse(p *domain.Position) positionResponse {
	return positionResponse{
		InstrumentID:      p.InstrumentID,
		VenueID:           p.VenueID,
		Quantity:          p.Quantity.String(),
		AverageCost:       p.AverageCost.String(),
		MarketPrice:       p.MarketPrice.String(),
		UnrealizedPnL:     p.UnrealizedPnL.String(),
		RealizedPnL:       p.RealizedPnL.String(),
		UnsettledQuantity: p.UnsettledQuantity.String(),
		SettledQuantity:   p.SettledQuantity.String(),
		AssetClass:        assetClassStr(p.AssetClass),
		QuoteCurrency:     p.QuoteCurrency,
		UpdatedAt:         p.UpdatedAt,
	}
}

// listPositions handles GET /api/v1/positions.
func (h *handler) listPositions(w http.ResponseWriter, r *http.Request) {
	positions, err := h.store.ListPositions(r.Context())
	if err != nil {
		apperror.WriteError(w, err)
		return
	}

	result := make([]positionResponse, 0, len(positions))
	for i := range positions {
		result = append(result, toPositionResponse(&positions[i]))
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(result)
}

// getPosition handles GET /api/v1/positions/{instrumentID}.
func (h *handler) getPosition(w http.ResponseWriter, r *http.Request) {
	instrumentID := chi.URLParam(r, "instrumentID")

	// Try to find position across any venue (we use empty string for venueID
	// since the REST API queries by instrument only).
	positions, err := h.store.ListPositions(r.Context())
	if err != nil {
		apperror.WriteError(w, err)
		return
	}

	var matched []positionResponse
	for i := range positions {
		if positions[i].InstrumentID == instrumentID {
			matched = append(matched, toPositionResponse(&positions[i]))
		}
	}

	if len(matched) == 0 {
		apperror.WriteError(w, apperror.ErrPositionNotFound)
		return
	}

	// If there's exactly one, return it directly; otherwise return the array.
	if len(matched) == 1 {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(matched[0])
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(matched)
}
