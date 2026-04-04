import type { PortfolioSummary, ExposureData } from "../../api/types";

export const mockPortfolioSummary: PortfolioSummary = {
  totalNav: "250000.00",
  totalPnl: "12500.00",
  dailyPnl: "1500.00",
  cash: "100000.00",
  availableCash: "95000.00",
  positionCount: 5,
};

export const mockExposure: ExposureData = {
  byAssetClass: [
    { assetClass: "equity", notional: "137500.00", percentage: 55 },
    { assetClass: "crypto", notional: "112500.00", percentage: 45 },
  ],
  byVenue: [
    { venueId: "alpaca", notional: "137500.00", percentage: 55 },
    { venueId: "binance", notional: "112500.00", percentage: 45 },
  ],
};
