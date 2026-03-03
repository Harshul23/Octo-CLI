import { Routes, Route, Navigate } from "react-router-dom";
import Homepage from "./pages/homepage.jsx";
import Entrypage from "./pages/entrypage.jsx";
import Docs from "./pages/docs-pages/docs.jsx";
import Layout from "./layouts/Layout.jsx";
import DocsLayout from "./layouts/DocsLayout.jsx";
import Overview from "./pages/docs-pages/overview.jsx";
import Quickstart from "./pages/docs-pages/quickstart.jsx";

function App() {
  return (
    <Routes>
      <Route path="/" element={<Entrypage />} />
      <Route element={<Layout />}>
        <Route path="/home" element={<Homepage />} />
        <Route path="/docs" element={<Docs />} />
        <Route path="*" element={<Navigate to="/" />} />
      </Route>
      <Route element={<DocsLayout />}>
        <Route path="/docs/overview" element={<Overview />} />
        <Route path="/docs/quickstart" element={<Quickstart />} />
        <Route path="*" element={<Navigate to="/" />} />
      </Route>
    </Routes>
  );
}

export default App;