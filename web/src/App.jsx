import { Routes, Route, Navigate } from "react-router-dom";
import DashboardLayout from "./layouts/DashboardLayout.jsx";
import Entrypage from "./pages/landing/entrypage.jsx";
import Homepage from "./pages/landing/homepage.jsx";
import LandingLayout from "./layouts/LandingLayout.jsx";
import Overview from "./pages/landing/docs-pages/overview.jsx";
import LandingDocsLayout from "./layouts/LandingDocsLayout.jsx";
import Dashboard from "./pages/Dashboard.jsx";
import Analyze from "./pages/Analyze.jsx";
import ProjectDetail from "./pages/ProjectDetail.jsx";
import Explore from "./pages/Explore.jsx";
import ExploreDetail from "./pages/ExploreDetail.jsx";
import Settings from "./pages/Settings.jsx";
import RepoSelector from "./pages/RepoSelector.jsx";

function App() {
  return (
    <Routes>
      <Route path="/" element={<Entrypage />} />
      <Route element={<LandingLayout />}>
        <Route path="/home" element={<Homepage />} />
      </Route>
      <Route element={<LandingDocsLayout />}>
        <Route path="/docs" element={<Navigate to="/docs/overview" replace />} />
        <Route path="/docs/overview" element={<Overview />} />
      </Route>
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
