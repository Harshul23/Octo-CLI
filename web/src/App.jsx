import { Routes, Route, Navigate } from "react-router-dom";
import DashboardLayout from "./layouts/DashboardLayout.jsx";
import Landing from "./pages/Landing.jsx";
import Dashboard from "./pages/Dashboard.jsx";
import Analyze from "./pages/Analyze.jsx";
import ProjectDetail from "./pages/ProjectDetail.jsx";
import Explore from "./pages/Explore.jsx";
import ExploreDetail from "./pages/ExploreDetail.jsx";
import Settings from "./pages/Settings.jsx";
import AuthCallback from "./pages/AuthCallback.jsx";
import RepoSelector from "./pages/RepoSelector.jsx";

function App() {
  return (
    <Routes>
      <Route path="/" element={<Landing />} />
      <Route path="/auth/callback" element={<AuthCallback />} />
      <Route element={<DashboardLayout />}>
        <Route path="/dashboard" element={<Dashboard />} />
        <Route path="/repos" element={<RepoSelector />} />
        <Route path="/analyze" element={<Analyze />} />
        <Route path="/projects/:id" element={<ProjectDetail />} />
        <Route path="/explore" element={<Explore />} />
        <Route path="/explore/:id" element={<ExploreDetail />} />
        <Route path="/settings" element={<Settings />} />
      </Route>
      <Route path="*" element={<Navigate to="/" />} />
    </Routes>
  );
}

export default App;
