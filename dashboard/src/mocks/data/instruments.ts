import type { Instrument } from "../../api/types";

export const mockInstruments: Instrument[] = [
  {
    id: "AAPL",
    symbol: "AAPL",
    name: "Apple Inc.",
    assetClass: "equity",
    baseCurrency: "USD",
    quoteCurrency: "USD",
    venueId: "sim-exchange",
  },
  {
    id: "BTC-USD",
    symbol: "BTC-USD",
    name: "Bitcoin / USD",
    assetClass: "crypto",
    baseCurrency: "BTC",
    quoteCurrency: "USD",
    venueId: "sim-exchange",
  },
  {
    id: "ETH-USD",
    symbol: "ETH-USD",
    name: "Ethereum / USD",
    assetClass: "crypto",
    baseCurrency: "ETH",
    quoteCurrency: "USD",
    venueId: "sim-exchange",
  },
  {
    id: "GOOG",
    symbol: "GOOG",
    name: "Alphabet Inc.",
    assetClass: "equity",
    baseCurrency: "USD",
    quoteCurrency: "USD",
    venueId: "sim-exchange",
  },
];
