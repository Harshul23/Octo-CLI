import { Routes, Route, Navigate } from "react-router-dom";
import Homepage from "./components/homepage.jsx";
import Entrypage from "./components/entrypage.jsx";
import Docs from "./components/Docs.jsx";
import Layout from "./components/Layout.jsx";

function App() {
  return (
    <Routes>
      <Route path="/" element={<Entrypage />} />
      <Route element={<Layout />}>
        <Route path="/home" element={<Homepage />} />
        <Route path="/docs" element={<Docs />} />
        <Route path="*" element={<Navigate to="/" />} />
      </Route>
    </Routes>
  );
}

export default App;