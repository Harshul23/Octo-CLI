import { Outlet } from "react-router-dom";
import Navbar from "../components/landing/navigation/navbar.jsx";
import Footer from "../components/landing/navigation/footer.jsx";

export default function LandingDocsLayout() {
  return (
    <div className="min-h-screen bg-black text-white flex flex-col">
      <Navbar />

      <div className="flex flex-1">
        <main className="flex-1 w-full px-12 py-10">
          <Outlet />
        </main>
      </div>

      <Footer />
    </div>
  );
}
