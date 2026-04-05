import { Outlet } from "react-router-dom";
import Navbar from "../components/landing/navigation/navbar.jsx";
import Footer from "../components/landing/navigation/footer.jsx";

export default function LandingLayout() {
  return (
    <div className="min-h-screen flex flex-col bg-black">
      <Navbar />
      <main className="flex-1">
        <Outlet />
      </main>
      <Footer />
    </div>
  );
}
