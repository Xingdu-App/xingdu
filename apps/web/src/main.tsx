import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import "./index.css";
import SessionGate from "./SessionGate.tsx";
import "./App.css";

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <SessionGate />
  </StrictMode>,
);
