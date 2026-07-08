import React from "react";
import ReactDOM from "react-dom/client";
import { BrowserRouter } from "react-router-dom";
import App from "./App";
import { AuthProvider } from "./lib/auth";
import { RefreshProvider } from "./lib/refresh";
import "./index.css";

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <BrowserRouter>
      <AuthProvider>
        <RefreshProvider>
          <App />
        </RefreshProvider>
      </AuthProvider>
    </BrowserRouter>
  </React.StrictMode>,
);
