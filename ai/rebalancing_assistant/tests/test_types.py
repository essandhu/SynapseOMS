"""Tests for rebalancing assistant types."""

from __future__ import annotations

import os
import sys

# Add risk-engine to path so we can import OptimizationConstraints
sys.path.insert(
    0,
    os.path.join(os.path.dirname(__file__), "..", "..", "..", "..", "risk-engine"),
)

from ai.rebalancing_assistant.types import ExtractedConstraints, RebalanceRequest


class TestRebalanceRequest:
    """Tests for RebalanceRequest dataclass."""

    def test_to_prompt_vars_returns_expected_keys(self) -> None:
        req = RebalanceRequest(
            user_input="Maximize risk-adjusted return",
            portfolio_summary="BTC 40%, ETH 30%, SOL 30%",
            available_instruments="BTC, ETH, SOL, AVAX",
        )
        result = req.to_prompt_vars()
        assert isinstance(result, dict)
        assert set(result.keys()) == {
            "user_input",
            "portfolio_summary",
            "available_instruments",
        }
        assert result["user_input"] == "Maximize risk-adjusted return"
        assert result["portfolio_summary"] == "BTC 40%, ETH 30%, SOL 30%"
        assert result["available_instruments"] == "BTC, ETH, SOL, AVAX"


class TestExtractedConstraints:
    """Tests for ExtractedConstraints dataclass."""

    def _full_data(self) -> dict:
        return {
            "objective": "maximize_sharpe",
            "target_return": 0.12,
            "risk_aversion": 2.0,
            "long_only": True,
            "max_single_weight": 0.25,
            "asset_class_bounds": {"equity": [0.3, 0.7], "crypto": [0.1, 0.4]},
            "sector_limits": {"tech": 0.5},
            "target_volatility": 0.15,
            "max_turnover_usd": 50000.0,
            "instruments_to_include": ["BTC", "ETH"],
            "instruments_to_exclude": ["DOGE"],
            "reasoning": "User wants diversified portfolio with moderate risk.",
        }

    def test_from_dict_full_data(self) -> None:
        data = self._full_data()
        ec = ExtractedConstraints.from_dict(data)

        assert ec.objective == "maximize_sharpe"
        assert ec.target_return == 0.12
        assert ec.risk_aversion == 2.0
        assert ec.long_only is True
        assert ec.max_single_weight == 0.25
        assert ec.asset_class_bounds == {"equity": [0.3, 0.7], "crypto": [0.1, 0.4]}
        assert ec.sector_limits == {"tech": 0.5}
        assert ec.target_volatility == 0.15
        assert ec.max_turnover_usd == 50000.0
        assert ec.instruments_to_include == ["BTC", "ETH"]
        assert ec.instruments_to_exclude == ["DOGE"]
        assert "diversified" in ec.reasoning

    def test_to_optimization_constraints_turnover_conversion(self) -> None:
        """max_turnover_usd / NAV = max_turnover (weight-based)."""
        data = self._full_data()
        data["max_turnover_usd"] = 10000.0
        ec = ExtractedConstraints.from_dict(data)

        nav = 100000.0
        oc = ec.to_optimization_constraints(nav)

        assert oc.max_turnover == 10000.0 / 100000.0  # 0.1

    def test_to_optimization_constraints_asset_class_bounds_list_to_tuple(
        self,
    ) -> None:
        """asset_class_bounds lists should be converted to tuples."""
        data = self._full_data()
        data["asset_class_bounds"] = {"equity": [0.2, 0.8], "crypto": [0.0, 0.3]}
        ec = ExtractedConstraints.from_dict(data)

        oc = ec.to_optimization_constraints(nav=100000.0)

        assert oc.asset_class_bounds is not None
        assert oc.asset_class_bounds["equity"] == (0.2, 0.8)
        assert oc.asset_class_bounds["crypto"] == (0.0, 0.3)

    def test_to_optimization_constraints_maps_all_fields(self) -> None:
        """Verify all relevant fields are mapped to OptimizationConstraints."""
        data = self._full_data()
        ec = ExtractedConstraints.from_dict(data)
        oc = ec.to_optimization_constraints(nav=200000.0)

        assert oc.risk_aversion == 2.0
        assert oc.long_only is True
        assert oc.max_single_weight == 0.25
        assert oc.sector_limits == {"tech": 0.5}
        assert oc.target_volatility == 0.15

    def test_to_optimization_constraints_none_turnover(self) -> None:
        """When max_turnover_usd is None, max_turnover should be None."""
        data = self._full_data()
        data["max_turnover_usd"] = None
        ec = ExtractedConstraints.from_dict(data)

        oc = ec.to_optimization_constraints(nav=100000.0)
        assert oc.max_turnover is None
