import { Navigate, Route, Routes } from "react-router-dom";
import { useAuth } from "./lib/auth";
import AppLayout from "./components/AppLayout";
import LoginPage from "./pages/LoginPage";
import OverviewPage from "./pages/OverviewPage";
import DevicesPage from "./pages/DevicesPage";
import DeviceDetailPage from "./pages/DeviceDetailPage";
import ClientsPage from "./pages/ClientsPage";
import LogsPage from "./pages/LogsPage";
import TrafficPage from "./pages/TrafficPage";
import WifiPage from "./pages/WifiPage";
import FirewallPage from "./pages/FirewallPage";
import AiChatPage from "./pages/AiChatPage";
import AlertsPage from "./pages/AlertsPage";
import TopologyPage from "./pages/TopologyPage";

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
        <Route path="/traffic" element={<TrafficPage />} />
        <Route path="/wifi" element={<WifiPage />} />
        <Route path="/devices" element={<DevicesPage />} />
        <Route path="/devices/:name" element={<DeviceDetailPage />} />
        <Route path="/clients" element={<ClientsPage />} />
        <Route path="/firewall" element={<FirewallPage />} />
        <Route path="/alerts" element={<AlertsPage />} />
        <Route path="/topology" element={<TopologyPage />} />
        <Route path="/logs" element={<LogsPage />} />
        <Route path="/ai" element={<AiChatPage />} />
        <Route path="/login" element={<Navigate to="/" replace />} />
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </AppLayout>
  );
}
