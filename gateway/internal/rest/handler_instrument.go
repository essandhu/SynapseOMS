package rest

import (
	"encoding/json"
	"net/http"

	"github.com/synapse-oms/gateway/internal/apperror"
	"github.com/synapse-oms/gateway/internal/domain"
)

// instrumentResponse is the JSON response for an instrument.
type instrumentResponse struct {
	ID              string   `json:"id"`
	Symbol          string   `json:"symbol"`
	Name            string   `json:"name"`
	AssetClass      string   `json:"asset_class"`
	QuoteCurrency   string   `json:"quote_currency"`
	BaseCurrency    string   `json:"base_currency,omitempty"`
	TickSize        string   `json:"tick_size"`
	LotSize         string   `json:"lot_size"`
	SettlementCycle string   `json:"settlement_cycle"`
	Venues          []string `json:"venues"`
	MarginRequired  string   `json:"margin_required"`
}

func toInstrumentResponse(inst *domain.Instrument) instrumentResponse {
	return instrumentResponse{
		ID:              inst.ID,
		Symbol:          inst.Symbol,
		Name:            inst.Name,
		AssetClass:      assetClassStr(inst.AssetClass),
		QuoteCurrency:   inst.QuoteCurrency,
		BaseCurrency:    inst.BaseCurrency,
		TickSize:        inst.TickSize.String(),
		LotSize:         inst.LotSize.String(),
		SettlementCycle: settlementCycleStr(inst.SettlementCycle),
		Venues:          inst.Venues,
		MarginRequired:  inst.MarginRequired.String(),
	}
}

// listInstruments handles GET /api/v1/instruments.
func (h *handler) listInstruments(w http.ResponseWriter, r *http.Request) {
	instruments, err := h.store.ListInstruments(r.Context())
	if err != nil {
		apperror.WriteError(w, err)
		return
	}

	result := make([]instrumentResponse, 0, len(instruments))
	for i := range instruments {
		result = append(result, toInstrumentResponse(&instruments[i]))
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(result)
}

func assetClassStr(a domain.AssetClass) string {
	switch a {
	case domain.AssetClassEquity:
		return "equity"
	case domain.AssetClassCrypto:
		return "crypto"
	case domain.AssetClassTokenizedSecurity:
		return "tokenized_security"
	case domain.AssetClassFuture:
		return "future"
	case domain.AssetClassOption:
		return "option"
	default:
		return "equity"
	}
}

func settlementCycleStr(c domain.SettlementCycle) string {
	switch c {
	case domain.SettlementT0:
		return "T+0"
	case domain.SettlementT1:
		return "T+1"
	case domain.SettlementT2:
		return "T+2"
	default:
		return "T+0"
	}
}
