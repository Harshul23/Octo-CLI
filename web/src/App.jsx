import { Routes, Route, Navigate } from "react-router-dom";
import Homepage from "./components/homepage.jsx";
import Entrypage from "./components/entrypage.jsx";
import Docs from "./components/docs.jsx";
import Layout from "./components/Layout.jsx";
import DocsLayout from "./components/DocsLayout.jsx";
import Install from "./components/install.jsx";

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
        <Route path="/docs/installation" element={<Install />} />
        <Route path="*" element={<Navigate to="/" />} />
      </Route>
    </Routes>
  );
}

export default App;