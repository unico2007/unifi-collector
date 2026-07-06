import { Navigate, Route, Routes } from "react-router-dom";
import { useAuth } from "./lib/auth";
import AppLayout from "./components/AppLayout";
import LoginPage from "./pages/LoginPage";
import OverviewPage from "./pages/OverviewPage";
import DevicesPage from "./pages/DevicesPage";
import ClientsPage from "./pages/ClientsPage";
import LogsPage from "./pages/LogsPage";

export default function App() {
  const { user } = useAuth();

  if (!user) {
    return (
      <Routes>
        <Route path="/login" element={<LoginPage />} />
        <Route path="*" element={<Navigate to="/login" replace />} />
      </Routes>
    );
  }

  return (
    <AppLayout>
      <Routes>
        <Route path="/" element={<OverviewPage />} />
        <Route path="/devices" element={<DevicesPage />} />
        <Route path="/clients" element={<ClientsPage />} />
        <Route path="/logs" element={<LogsPage />} />
        <Route path="/login" element={<Navigate to="/" replace />} />
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </AppLayout>
  );
}
