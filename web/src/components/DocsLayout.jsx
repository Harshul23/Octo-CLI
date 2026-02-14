import { Outlet } from "react-router-dom";
import Navbar from "./navbar";
import Footer from "./footer";
import Sidebar from "./sidebar";

export default function DocsLayout() {
  return (
    <div className="min-h-screen bg-black text-white flex flex-col">
      <Navbar />

      <div className="flex flex-1">
        <Sidebar />
        
        <main className="flex-1 px-12 py-10">
          <Outlet />
        </main>
      </div>

      <Footer />
    </div>
  );
}