import { Outlet } from "react-router-dom";
import Navbar from "../components/navigation/navbar";
import Footer from "../components/navigation/footer";
import Sidebar from "../components/navigation/sidebar";

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