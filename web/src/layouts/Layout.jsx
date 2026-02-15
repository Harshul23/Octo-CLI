import { Outlet } from "react-router-dom";
import Navbar from "../components/navigation/navbar.jsx";
import Footer from "../components/navigation/footer.jsx";

export default function Layout() {
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