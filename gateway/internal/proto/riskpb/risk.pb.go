// Package riskpb provides Go types matching proto/risk/risk.proto.
//
// These types use manual protobuf wire encoding via google.golang.org/protobuf/encoding/protowire
// so they are wire-compatible with the Python gRPC server's generated stubs.
//
// When buf tooling is available, regenerate with: ./scripts/proto-gen.sh
package riskpb

import (
	"google.golang.org/protobuf/encoding/protowire"
)

// PreTradeRiskRequest matches synapse.risk.PreTradeRiskRequest.
type PreTradeRiskRequest struct {
	OrderId      string
	InstrumentId string
	Side         string
	Quantity     string
	Price        string
	AssetClass   string
	VenueId      string
}

func (m *PreTradeRiskRequest) MarshalProto() ([]byte, error) {
	var b []byte
	if m.OrderId != "" {
		b = protowire.AppendTag(b, 1, protowire.BytesType)
		b = protowire.AppendString(b, m.OrderId)
	}
	if m.InstrumentId != "" {
		b = protowire.AppendTag(b, 2, protowire.BytesType)
		b = protowire.AppendString(b, m.InstrumentId)
	}
	if m.Side != "" {
		b = protowire.AppendTag(b, 3, protowire.BytesType)
		b = protowire.AppendString(b, m.Side)
	}
	if m.Quantity != "" {
		b = protowire.AppendTag(b, 4, protowire.BytesType)
		b = protowire.AppendString(b, m.Quantity)
	}
	if m.Price != "" {
		b = protowire.AppendTag(b, 5, protowire.BytesType)
		b = protowire.AppendString(b, m.Price)
	}
	if m.AssetClass != "" {
		b = protowire.AppendTag(b, 6, protowire.BytesType)
		b = protowire.AppendString(b, m.AssetClass)
	}
	if m.VenueId != "" {
		b = protowire.AppendTag(b, 7, protowire.BytesType)
		b = protowire.AppendString(b, m.VenueId)
	}
	return b, nil
}

func (m *PreTradeRiskRequest) UnmarshalProto(data []byte) error {
	for len(data) > 0 {
		num, typ, n := protowire.ConsumeTag(data)
		if n < 0 {
			return protowire.ParseError(n)
		}
		data = data[n:]

		switch typ {
		case protowire.BytesType:
			v, n := protowire.ConsumeString(data)
			if n < 0 {
				return protowire.ParseError(n)
			}
			data = data[n:]
			switch num {
			case 1:
				m.OrderId = v
			case 2:
				m.InstrumentId = v
			case 3:
				m.Side = v
			case 4:
				m.Quantity = v
			case 5:
				m.Price = v
			case 6:
				m.AssetClass = v
			case 7:
				m.VenueId = v
			}
		case protowire.VarintType:
			_, n := protowire.ConsumeVarint(data)
			if n < 0 {
				return protowire.ParseError(n)
			}
			data = data[n:]
		default:
			n := protowire.ConsumeFieldValue(num, typ, data)
			if n < 0 {
				return protowire.ParseError(n)
			}
			data = data[n:]
		}
	}
	return nil
}

// RiskCheck matches synapse.risk.RiskCheck.
type RiskCheck struct {
	Name      string
	Passed    bool
	Message   string
	Threshold string
	Actual    string
}

func (m *RiskCheck) MarshalProto() ([]byte, error) {
	var b []byte
	if m.Name != "" {
		b = protowire.AppendTag(b, 1, protowire.BytesType)
		b = protowire.AppendString(b, m.Name)
	}
	if m.Passed {
		b = protowire.AppendTag(b, 2, protowire.VarintType)
		b = protowire.AppendVarint(b, 1)
	}
	if m.Message != "" {
		b = protowire.AppendTag(b, 3, protowire.BytesType)
		b = protowire.AppendString(b, m.Message)
	}
	if m.Threshold != "" {
		b = protowire.AppendTag(b, 4, protowire.BytesType)
		b = protowire.AppendString(b, m.Threshold)
	}
	if m.Actual != "" {
		b = protowire.AppendTag(b, 5, protowire.BytesType)
		b = protowire.AppendString(b, m.Actual)
	}
	return b, nil
}

func (m *RiskCheck) UnmarshalProto(data []byte) error {
	for len(data) > 0 {
		num, typ, n := protowire.ConsumeTag(data)
		if n < 0 {
			return protowire.ParseError(n)
		}
		data = data[n:]

		switch typ {
		case protowire.BytesType:
			v, n := protowire.ConsumeString(data)
			if n < 0 {
				return protowire.ParseError(n)
			}
			data = data[n:]
			switch num {
			case 1:
				m.Name = v
			case 3:
				m.Message = v
			case 4:
				m.Threshold = v
			case 5:
				m.Actual = v
			}
		case protowire.VarintType:
			v, n := protowire.ConsumeVarint(data)
			if n < 0 {
				return protowire.ParseError(n)
			}
			data = data[n:]
			if num == 2 {
				m.Passed = v != 0
			}
		default:
			n := protowire.ConsumeFieldValue(num, typ, data)
			if n < 0 {
				return protowire.ParseError(n)
			}
			data = data[n:]
		}
	}
	return nil
}

// PreTradeRiskResponse matches synapse.risk.PreTradeRiskResponse.
type PreTradeRiskResponse struct {
	Approved              bool
	RejectReason          string
	Checks                []*RiskCheck
	PortfolioVarBefore    string
	PortfolioVarAfter     string
	ComputedAtMs          int64
	OrderId               string
	ConcentrationWarning  string
	MaxLossEstimate       string
}

func (m *PreTradeRiskResponse) MarshalProto() ([]byte, error) {
	var b []byte
	if m.Approved {
		b = protowire.AppendTag(b, 1, protowire.VarintType)
		b = protowire.AppendVarint(b, 1)
	}
	if m.RejectReason != "" {
		b = protowire.AppendTag(b, 2, protowire.BytesType)
		b = protowire.AppendString(b, m.RejectReason)
	}
	for _, c := range m.Checks {
		cb, err := c.MarshalProto()
		if err != nil {
			return nil, err
		}
		b = protowire.AppendTag(b, 3, protowire.BytesType)
		b = protowire.AppendBytes(b, cb)
	}
	if m.PortfolioVarBefore != "" {
		b = protowire.AppendTag(b, 4, protowire.BytesType)
		b = protowire.AppendString(b, m.PortfolioVarBefore)
	}
	if m.PortfolioVarAfter != "" {
		b = protowire.AppendTag(b, 5, protowire.BytesType)
		b = protowire.AppendString(b, m.PortfolioVarAfter)
	}
	if m.ComputedAtMs != 0 {
		b = protowire.AppendTag(b, 6, protowire.VarintType)
		b = protowire.AppendVarint(b, uint64(m.ComputedAtMs))
	}
	if m.OrderId != "" {
		b = protowire.AppendTag(b, 7, protowire.BytesType)
		b = protowire.AppendString(b, m.OrderId)
	}
	if m.ConcentrationWarning != "" {
		b = protowire.AppendTag(b, 8, protowire.BytesType)
		b = protowire.AppendString(b, m.ConcentrationWarning)
	}
	if m.MaxLossEstimate != "" {
		b = protowire.AppendTag(b, 9, protowire.BytesType)
		b = protowire.AppendString(b, m.MaxLossEstimate)
	}
	return b, nil
}

func (m *PreTradeRiskResponse) UnmarshalProto(data []byte) error {
	for len(data) > 0 {
		num, typ, n := protowire.ConsumeTag(data)
		if n < 0 {
			return protowire.ParseError(n)
		}
		data = data[n:]

		switch typ {
		case protowire.VarintType:
			v, n := protowire.ConsumeVarint(data)
			if n < 0 {
				return protowire.ParseError(n)
			}
			data = data[n:]
			switch num {
			case 1:
				m.Approved = v != 0
			case 6:
				m.ComputedAtMs = int64(v)
			}
		case protowire.BytesType:
			v, n := protowire.ConsumeBytes(data)
			if n < 0 {
				return protowire.ParseError(n)
			}
			data = data[n:]
			switch num {
			case 2:
				m.RejectReason = string(v)
			case 3:
				c := &RiskCheck{}
				if err := c.UnmarshalProto(v); err != nil {
					return err
				}
				m.Checks = append(m.Checks, c)
			case 4:
				m.PortfolioVarBefore = string(v)
			case 5:
				m.PortfolioVarAfter = string(v)
			case 7:
				m.OrderId = string(v)
			case 8:
				m.ConcentrationWarning = string(v)
			case 9:
				m.MaxLossEstimate = string(v)
			}
		default:
			n := protowire.ConsumeFieldValue(num, typ, data)
			if n < 0 {
				return protowire.ParseError(n)
			}
			data = data[n:]
		}
	}
	return nil
}
