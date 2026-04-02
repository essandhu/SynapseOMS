import { BrowserRouter, Route, Routes } from "react-router";
import { TerminalLayout } from "./components/TerminalLayout";
import { PortfolioView } from "./views/PortfolioView";

function BlotterView() {
  return (
    <div className="font-mono text-sm text-text-secondary">Blotter</div>
  );
}

export function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route element={<TerminalLayout />}>
          <Route index element={<BlotterView />} />
          <Route path="portfolio" element={<PortfolioView />} />
        </Route>
      </Routes>
    </BrowserRouter>
  );
}
