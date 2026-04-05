package rest

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/shopspring/decimal"
	"github.com/synapse-oms/gateway/internal/adapter"
	"github.com/synapse-oms/gateway/internal/apperror"
	"github.com/synapse-oms/gateway/internal/domain"
	"github.com/synapse-oms/gateway/internal/metrics"
	"github.com/synapse-oms/gateway/internal/store"
)

// submitOrderRequest is the JSON request body for POST /orders.
type submitOrderRequest struct {
	InstrumentID  string `json:"instrument_id"`
	Side          string `json:"side"`
	Type          string `json:"type"`
	Quantity      string `json:"quantity"`
	Price         string `json:"price"`
	ClientOrderID string `json:"client_order_id"`
	VenueID       string `json:"venue_id"`
}

// orderResponse is the JSON response for an order.
type orderResponse struct {
	ID              string         `json:"id"`
	ClientOrderID   string         `json:"client_order_id,omitempty"`
	InstrumentID    string         `json:"instrument_id"`
	Side            string         `json:"side"`
	Type            string         `json:"type"`
	Quantity        string         `json:"quantity"`
	Price           string         `json:"price"`
	FilledQuantity  string         `json:"filled_quantity"`
	AveragePrice    string         `json:"average_price"`
	Status          string         `json:"status"`
	AssetClass      string         `json:"asset_class,omitempty"`
	VenueID         string         `json:"venue_id,omitempty"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	Fills           []fillResponse `json:"fills,omitempty"`
}

type fillResponse struct {
	ID        string    `json:"id"`
	OrderID   string    `json:"order_id"`
	VenueID   string    `json:"venue_id"`
	Quantity  string    `json:"quantity"`
	Price     string    `json:"price"`
	Fee       string    `json:"fee"`
	FeeAsset  string    `json:"fee_asset"`
	Timestamp time.Time `json:"timestamp"`
}

func toOrderResponse(o *domain.Order) orderResponse {
	return orderResponse{
		ID:             string(o.ID),
		ClientOrderID:  o.ClientOrderID,
		InstrumentID:   o.InstrumentID,
		Side:           sideToString(o.Side),
		Type:           typeToString(o.Type),
		Quantity:       o.Quantity.String(),
		Price:          o.Price.String(),
		FilledQuantity: o.FilledQuantity.String(),
		AveragePrice:   o.AveragePrice.String(),
		Status:         statusToString(o.Status),
		AssetClass:     assetClassStr(o.AssetClass),
		VenueID:        o.VenueID,
		CreatedAt:      o.CreatedAt,
		UpdatedAt:      o.UpdatedAt,
	}
}

func toFillResponse(f domain.Fill) fillResponse {
	return fillResponse{
		ID:        f.ID,
		OrderID:   string(f.OrderID),
		VenueID:   f.VenueID,
		Quantity:  f.Quantity.String(),
		Price:     f.Price.String(),
		Fee:       f.Fee.String(),
		FeeAsset:  f.FeeAsset,
		Timestamp: f.Timestamp,
	}
}

// submitOrder handles POST /api/v1/orders.
func (h *handler) submitOrder(w http.ResponseWriter, r *http.Request) {
	var req submitOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apperror.WriteError(w, &apperror.AppError{
			Code:       "VALIDATION_ERROR",
			Message:    "invalid JSON body",
			HTTPStatus: http.StatusBadRequest,
		})
		return
	}

	// Validate instrument_id
	if strings.TrimSpace(req.InstrumentID) == "" {
		apperror.WriteError(w, &apperror.AppError{
			Code:       "VALIDATION_ERROR",
			Message:    "instrument_id is required",
			HTTPStatus: http.StatusBadRequest,
		})
		return
	}

	// Validate side
	if req.Side != "buy" && req.Side != "sell" {
		apperror.WriteError(w, &apperror.AppError{
			Code:       "VALIDATION_ERROR",
			Message:    "side must be 'buy' or 'sell'",
			HTTPStatus: http.StatusBadRequest,
		})
		return
	}

	// Validate type
	if req.Type != "market" && req.Type != "limit" {
		apperror.WriteError(w, &apperror.AppError{
			Code:       "VALIDATION_ERROR",
			Message:    "type must be 'market' or 'limit'",
			HTTPStatus: http.StatusBadRequest,
		})
		return
	}

	// Parse and validate quantity
	quantity, err := decimal.NewFromString(req.Quantity)
	if err != nil || !quantity.IsPositive() {
		apperror.WriteError(w, &apperror.AppError{
			Code:       "VALIDATION_ERROR",
			Message:    "quantity must be a positive number",
			HTTPStatus: http.StatusBadRequest,
		})
		return
	}

	// Parse price
	price, err := decimal.NewFromString(req.Price)
	if err != nil {
		price = decimal.Zero
	}

	// Validate limit price
	if req.Type == "limit" && !price.IsPositive() {
		apperror.WriteError(w, &apperror.AppError{
			Code:       "VALIDATION_ERROR",
			Message:    "price must be positive for limit orders",
			HTTPStatus: http.StatusBadRequest,
		})
		return
	}

	// Check instrument exists
	inst, err := h.store.GetInstrument(r.Context(), req.InstrumentID)
	if err != nil {
		apperror.WriteError(w, apperror.ErrInstrumentNotFound)
		return
	}

	// Build domain order
	side := domain.SideBuy
	if req.Side == "sell" {
		side = domain.SideSell
	}
	orderType := domain.OrderTypeMarket
	if req.Type == "limit" {
		orderType = domain.OrderTypeLimit
	}

	// Determine venue ID: empty or "smart" triggers smart routing (leave VenueID empty);
	// a specific venue ID triggers venue-preference strategy.
	venueID := strings.TrimSpace(req.VenueID)
	if strings.EqualFold(venueID, "smart") {
		venueID = ""
	}

	// Validate venue is connected when explicitly specified
	if venueID != "" {
		if err := adapter.CheckVenueReady(venueID); err != nil {
			apperror.WriteError(w, &apperror.AppError{
				Code:       "VENUE_NOT_CONNECTED",
				Message:    "venue is not connected: " + venueID,
				HTTPStatus: http.StatusBadRequest,
			})
			return
		}

		// Validate venue supports the instrument's asset class
		if provider, ok := adapter.GetInstance(venueID); ok {
			supported := false
			for _, ac := range provider.SupportedAssetClasses() {
				if ac == inst.AssetClass {
					supported = true
					break
				}
			}
			if !supported {
				apperror.WriteError(w, &apperror.AppError{
					Code:       "VENUE_ASSET_MISMATCH",
					Message:    "venue " + venueID + " does not support asset class " + assetClassToString(inst.AssetClass),
					HTTPStatus: http.StatusBadRequest,
				})
				return
			}
		}
	}

	// For smart routing, verify at least one compatible venue is connected
	if venueID == "" {
		connected := adapter.ListConnected()
		hasCompatible := false
		for _, p := range connected {
			for _, ac := range p.SupportedAssetClasses() {
				if ac == inst.AssetClass {
					hasCompatible = true
					break
				}
			}
			if hasCompatible {
				break
			}
		}
		if !hasCompatible {
			apperror.WriteError(w, &apperror.AppError{
				Code:       "NO_CONNECTED_VENUE",
				Message:    "no connected venue supports " + assetClassToString(inst.AssetClass),
				HTTPStatus: http.StatusBadRequest,
			})
			return
		}
	}

	// Sell-side validation: ensure sufficient position is held
	if side == domain.SideSell {
		positions, listErr := h.store.ListPositions(r.Context())
		if listErr != nil {
			apperror.WriteError(w, &apperror.AppError{
				Code:       "INTERNAL_ERROR",
				Message:    "failed to check positions",
				HTTPStatus: http.StatusInternalServerError,
			})
			return
		}

		var netQty decimal.Decimal
		for _, pos := range positions {
			if pos.InstrumentID == req.InstrumentID {
				netQty = netQty.Add(pos.Quantity)
			}
		}

		if !netQty.IsPositive() {
			apperror.WriteError(w, &apperror.AppError{
				Code:       "INSUFFICIENT_POSITION",
				Message:    "no position held for " + req.InstrumentID,
				HTTPStatus: http.StatusBadRequest,
			})
			return
		}
		if quantity.GreaterThan(netQty) {
			apperror.WriteError(w, &apperror.AppError{
				Code:       "INSUFFICIENT_POSITION",
				Message:    "sell quantity exceeds held position (" + netQty.String() + " available)",
				HTTPStatus: http.StatusBadRequest,
			})
			return
		}
	}

	clientOrderID := strings.TrimSpace(req.ClientOrderID)
	if clientOrderID == "" {
		clientOrderID = newClientOrderID()
	}

	order := &domain.Order{
		InstrumentID:    req.InstrumentID,
		ClientOrderID:   clientOrderID,
		Side:            side,
		Type:            orderType,
		Quantity:        quantity,
		Price:           price,
		VenueID:         venueID,
		AssetClass:      inst.AssetClass,
		SettlementCycle: inst.SettlementCycle,
	}

	submitStart := time.Now()
	if err := h.pipeline.Submit(r.Context(), order); err != nil {
		apperror.WriteError(w, err)
		return
	}
	metrics.OrderLatencySeconds.Observe(time.Since(submitStart).Seconds())

	resp := toOrderResponse(order)
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(resp)
}

// cancelOrder handles DELETE /api/v1/orders/{id}.
func (h *handler) cancelOrder(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	order, err := h.store.GetOrder(r.Context(), domain.OrderID(id))
	if err != nil {
		apperror.WriteError(w, apperror.ErrOrderNotFound)
		return
	}

	if err := order.ApplyTransition(domain.OrderStatusCanceled); err != nil {
		apperror.WriteError(w, apperror.ErrInvalidTransition)
		return
	}

	resp := toOrderResponse(order)
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// listOrders handles GET /api/v1/orders.
func (h *handler) listOrders(w http.ResponseWriter, r *http.Request) {
	filter := store.OrderFilter{}

	if s := r.URL.Query().Get("status"); s != "" {
		status := stringToStatus(s)
		filter.Status = &status
	}
	if inst := r.URL.Query().Get("instrument_id"); inst != "" {
		filter.InstrumentID = &inst
	}

	orders, err := h.store.ListOrders(r.Context(), filter)
	if err != nil {
		apperror.WriteError(w, err)
		return
	}

	result := make([]orderResponse, 0, len(orders))
	for _, o := range orders {
		result = append(result, toOrderResponse(o))
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(result)
}

// getOrder handles GET /api/v1/orders/{id}.
func (h *handler) getOrder(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	order, err := h.store.GetOrder(r.Context(), domain.OrderID(id))
	if err != nil {
		apperror.WriteError(w, apperror.ErrOrderNotFound)
		return
	}

	resp := toOrderResponse(order)

	// Attach fills
	fills, err := h.store.ListFillsByOrder(r.Context(), order.ID)
	if err == nil && len(fills) > 0 {
		resp.Fills = make([]fillResponse, 0, len(fills))
		for _, f := range fills {
			resp.Fills = append(resp.Fills, toFillResponse(f))
		}
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// --- string conversion helpers ---

func sideToString(s domain.OrderSide) string {
	switch s {
	case domain.SideBuy:
		return "buy"
	case domain.SideSell:
		return "sell"
	default:
		return "buy"
	}
}

func typeToString(t domain.OrderType) string {
	switch t {
	case domain.OrderTypeMarket:
		return "market"
	case domain.OrderTypeLimit:
		return "limit"
	case domain.OrderTypeStopLimit:
		return "stop_limit"
	default:
		return "market"
	}
}

func statusToString(s domain.OrderStatus) string {
	switch s {
	case domain.OrderStatusNew:
		return "new"
	case domain.OrderStatusAcknowledged:
		return "acknowledged"
	case domain.OrderStatusPartiallyFilled:
		return "partially_filled"
	case domain.OrderStatusFilled:
		return "filled"
	case domain.OrderStatusCanceled:
		return "canceled"
	case domain.OrderStatusRejected:
		return "rejected"
	default:
		return "new"
	}
}

// newClientOrderID generates a unique client order ID when one is not provided.
func newClientOrderID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return fmt.Sprintf("auto-%x", b)
}

func stringToStatus(s string) domain.OrderStatus {
	switch s {
	case "new":
		return domain.OrderStatusNew
	case "acknowledged":
		return domain.OrderStatusAcknowledged
	case "partially_filled":
		return domain.OrderStatusPartiallyFilled
	case "filled":
		return domain.OrderStatusFilled
	case "canceled":
		return domain.OrderStatusCanceled
	case "rejected":
		return domain.OrderStatusRejected
	default:
		return domain.OrderStatusNew
	}
}
