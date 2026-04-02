import { BrowserRouter, Route, Routes } from "react-router";
import { TerminalLayout } from "./components/TerminalLayout";
import { PortfolioView } from "./views/PortfolioView";
import { BlotterView } from "./views/BlotterView";

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
