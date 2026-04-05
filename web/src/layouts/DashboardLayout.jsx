import { Outlet, NavLink, useNavigate, useLocation } from "react-router-dom";
import {
  LayoutDashboard,
  Compass,
  LogOut,
  Menu,
  Github,
  Zap,
  ChevronLeft,
  ChevronRight,
  MoreVertical,
  ChevronUp,
} from "lucide-react";
import { useState, useEffect, useRef } from "react";
import { Button } from "../components/ui/Button.jsx";
import OctoLogo from "../components/ui/OctoLogo.jsx";
import { Badge } from "../components/ui/Badge.jsx";
import { useAuth } from "../contexts/AuthContext";
import { MdAutoFixHigh } from "react-icons/md";
import { GoTelescopeFill } from "react-icons/go";

const RepoIcon = ({ size, className }) => (
  <svg
    aria-hidden="true"
    focusable="false"
    className={className}
    viewBox="0 0 16 16"
    width={size || 16}
    height={size || 16}
    fill="currentColor"
    style={{ verticalAlign: "text-bottom" }}
  >
    <path d="M2 2.5A2.5 2.5 0 0 1 4.5 0h8.75a.75.75 0 0 1 .75.75v12.5a.75.75 0 0 1-.75.75h-2.5a.75.75 0 0 1 0-1.5h1.75v-2h-8a1 1 0 0 0-.714 1.7.75.75 0 1 1-1.072 1.05A2.495 2.495 0 0 1 2 11.5Zm10.5-1h-8a1 1 0 0 0-1 1v6.708A2.486 2.486 0 0 1 4.5 9h8ZM5 12.25a.25.25 0 0 1 .25-.25h3.5a.25.25 0 0 1 .25.25v3.25a.25.25 0 0 1-.4.2l-1.45-1.087a.249.249 0 0 0-.3 0L5.4 15.7a.25.25 0 0 1-.4-.2Z"></path>
  </svg>
);

const settings = ({ size, className }) => (
  <svg
    xmlns="http://www.w3.org/2000/svg"
    viewBox="0 0 24 24"
    fill="currentColor"
    width={size || 16}
    height={size || 16}
  >
    <path d="M5.33409 4.54491C6.3494 3.63637 7.55145 2.9322 8.87555 2.49707C9.60856 3.4128 10.7358 3.99928 12 3.99928C13.2642 3.99928 14.3914 3.4128 15.1245 2.49707C16.4486 2.9322 17.6506 3.63637 18.6659 4.54491C18.2405 5.637 18.2966 6.90531 18.9282 7.99928C19.5602 9.09388 20.6314 9.77679 21.7906 9.95392C21.9279 10.6142 22 11.2983 22 11.9993C22 12.7002 21.9279 13.3844 21.7906 14.0446C20.6314 14.2218 19.5602 14.9047 18.9282 15.9993C18.2966 17.0932 18.2405 18.3616 18.6659 19.4536C17.6506 20.3622 16.4486 21.0664 15.1245 21.5015C14.3914 20.5858 13.2642 19.9993 12 19.9993C10.7358 19.9993 9.60856 20.5858 8.87555 21.5015C7.55145 21.0664 6.3494 20.3622 5.33409 19.4536C5.75952 18.3616 5.7034 17.0932 5.0718 15.9993C4.43983 14.9047 3.36862 14.2218 2.20935 14.0446C2.07212 13.3844 2 12.7002 2 11.9993C2 11.2983 2.07212 10.6142 2.20935 9.95392C3.36862 9.77679 4.43983 9.09388 5.0718 7.99928C5.7034 6.90531 5.75952 5.637 5.33409 4.54491ZM13.5 14.5974C14.9349 13.7689 15.4265 11.9342 14.5981 10.4993C13.7696 9.0644 11.9349 8.57277 10.5 9.4012C9.06512 10.2296 8.5735 12.0644 9.40192 13.4993C10.2304 14.9342 12.0651 15.4258 13.5 14.5974Z"></path>
  </svg>
);

const analyze = ({ size, className }) => <MdAutoFixHigh />;

const telescope = ({ size, className }) => <GoTelescopeFill />;

const navItems = [
  { to: "/dashboard", label: "Dashboard", icon: LayoutDashboard },
  { to: "/repos", label: "Repositories", icon: RepoIcon },
  { to: "/analyze", label: "Analyze", icon: analyze },
  { to: "/explore", label: "Explore", icon: telescope },
  { to: "/settings", label: "Settings", icon: settings },
];

export default function DashboardLayout() {
  const { user, signOut, isAuthenticated, signInWithGitHub, isLoading } =
    useAuth();
  const [sidebarOpen, setSidebarOpen] = useState(false);
  const [isExpanded, setIsExpanded] = useState(true);
  const [userMenuOpen, setUserMenuOpen] = useState(false);
  const navigate = useNavigate();
  const location = useLocation();
  const userMenuRef = useRef(null);

  useEffect(() => {
    if (!isLoading && !isAuthenticated) {
      navigate("/");
    }
  }, [isAuthenticated, isLoading, navigate]);

  const handleSignOut = async () => {
    await signOut();
    navigate("/");
  };

  useEffect(() => {
    const handleClickOutside = (event) => {
      if (userMenuRef.current && !userMenuRef.current.contains(event.target)) {
        setUserMenuOpen(false);
      }
    };
    document.addEventListener("mousedown", handleClickOutside);
    return () => document.removeEventListener("mousedown", handleClickOutside);
  }, []);

  if (isLoading) {
    return (
      <div className="min-h-screen bg-[#060606] text-white flex items-center justify-center">
        <div className="flex flex-col items-center gap-4">
          <Zap className="h-8 w-8 text-indigo-500 animate-pulse" />
          <p className="text-neutral-400">Loading Octo...</p>
        </div>
      </div>
    );
  }

  if (!isAuthenticated) {
    return null; // Will redirect in useEffect
  }

  return (
    <div className="flex h-screen overflow-hidden bg-background text-foreground">
      {/* Mobile sidebar overlay */}
      {sidebarOpen && (
        <div
          className="fixed inset-0 z-40 bg-black/60 lg:hidden backdrop-blur-sm transition-opacity"
          onClick={() => setSidebarOpen(false)}
        />
      )}

      {/* Sidebar */}
      <aside
        className={`fixed lg:static inset-y-0 left-0 z-50 bg-[#0A0A0A] flex flex-col transition-all duration-300 ease-in-out
          ${sidebarOpen ? "translate-x-0" : "-translate-x-full lg:translate-x-0"}
          ${isExpanded ? "w-64" : "w-20"}
        `}
      >
        {/* Logo Header */}
        <div
          className={`flex items-center h-18 px-4 shrink-0 transition-all ${isExpanded ? "justify-between" : "justify-center"}`}
        >
          <div className="flex items-center gap-3 overflow-hidden">
            <OctoLogo size={28} className="shrink-0 text-white" />
            {isExpanded && (
              <div className="flex items-center gap-2">
                <span className="text-xl font-bold tracking-tight text-white">
                  Octo
                </span>
                <Badge
                  variant="secondary"
                  className="px-1.5 py-0 text-[9px] bg-neutral-800 text-neutral-300 border-none"
                >
                  BETA
                </Badge>
              </div>
            )}
          </div>
        </div>

        {/* Nav Items */}
        <nav className="flex-1 px-3 border-transparent py-4 space-y-1.5 overflow-y-auto overflow-x-hidden">
          {navItems.map((item) => {
            const isActive = location.pathname.startsWith(item.to);
            return (
              <NavLink
                key={item.to}
                to={item.to}
                onClick={() => setSidebarOpen(false)}
                className={`
                  flex items-center gap-3 px-3 py-2 rounded-md font-medium transition-all duration-200 group
                  ${
                    isActive
                      ? "bg-neutral-800/80 text-white shadow-sm"
                      : "text-neutral-400 hover:text-neutral-100 hover:bg-neutral-800/50"
                  }
                  ${isExpanded ? "" : "justify-center"}
                `}
                title={!isExpanded ? item.label : undefined}
              >
                <item.icon
                  size={18}
                  strokeWidth={2}
                  fill={isActive ? "currentColor" : "none"}
                  className={`shrink-0 transition-colors ${isActive ? "text-white" : "text-neutral-400 group-hover:text-neutral-100"}`}
                />
                {isExpanded && <span className="truncate">{item.label}</span>}
              </NavLink>
            );
          })}
        </nav>

        {/* User Footer */}
        <div
          className="p-3 shrink-0 mt-auto bg-[#0A0A0A] relative"
          ref={userMenuRef}
        >
          {isAuthenticated ? (
            <>
              <button
                onClick={() => setUserMenuOpen(!userMenuOpen)}
                className={`w-full flex items-center p-2 rounded-md transition-colors hover:bg-neutral-800/80
                  ${isExpanded ? "justify-between" : "justify-center"}
                  ${userMenuOpen ? "bg-neutral-800/80" : ""}
                `}
              >
                <div className="flex items-center gap-3 overflow-hidden">
                  {user?.user_metadata?.avatar_url ? (
                    <img
                      src={user.user_metadata.avatar_url}
                      alt="Avatar"
                      className="w-8 h-8 rounded-full ring-1 ring-neutral-700 shrink-0"
                    />
                  ) : (
                    <div className="flex items-center justify-center w-8 h-8 text-xs font-medium rounded-full bg-neutral-800 text-neutral-300 ring-1 ring-neutral-700 shrink-0">
                      {user?.email?.[0]?.toUpperCase() || "?"}
                    </div>
                  )}
                  {isExpanded && (
                    <div className="flex-1 min-w-0 text-left">
                      <p className="text-sm font-medium text-neutral-200 truncate">
                        {user?.user_metadata?.full_name ||
                          user?.email?.split("@")[0]}
                      </p>
                    </div>
                  )}
                </div>
                {isExpanded && (
                  <ChevronUp
                    size={16}
                    className={`text-neutral-500 transition-transform ${userMenuOpen ? "rotate-180" : ""}`}
                  />
                )}
              </button>

              {/* Popover Menu */}
              {userMenuOpen && (
                <div
                  className={`absolute bottom-full left-0 mb-2 bg-[#141414] border border-[#333] rounded-lg shadow-xl py-1 z-50
                  ${isExpanded ? "w-full left-3 right-3" : "w-48 left-full ml-4"}
                `}
                >
                  {isExpanded && (
                    <div className="px-3 py-2 border-b border-[#333] mb-1">
                      <p className="text-xs text-neutral-400 truncate">
                        {user?.email}
                      </p>
                    </div>
                  )}
                  <button
                    onClick={handleSignOut}
                    className="flex items-center w-full px-3 py-2 text-sm text-neutral-300 hover:text-white hover:bg-neutral-800 transition-colors group"
                  >
                    <LogOut
                      size={16}
                      className="mr-2 text-neutral-400 group-hover:text-red-400 transition-colors"
                    />
                    Sign Out
                  </button>
                </div>
              )}
            </>
          ) : (
            <Button
              onClick={signInWithGitHub}
              variant="secondary"
              className={`w-full transition-all ${isExpanded ? "justify-start px-4" : "justify-center px-0"} bg-neutral-800 hover:bg-neutral-700 text-white border-0`}
              title={!isExpanded ? "Sign in with GitHub" : undefined}
            >
              <Github size={18} className={isExpanded ? "mr-2" : ""} />
              {isExpanded && "Sign in"}
            </Button>
          )}
        </div>
      </aside>

      {/* Main content */}
      <div className="flex flex-col flex-1 min-w-0 overflow-hidden bg-[#0A0A0A]">
        {/* Top bar (mobile) */}
        <header className="flex items-center justify-between px-4 py-3 border-b border-[#222] lg:hidden bg-[#0A0A0A]">
          <button
            onClick={() => setSidebarOpen(true)}
            className="p-2 -ml-2 text-neutral-400 hover:text-white rounded-md hover:bg-neutral-800 transition-colors"
          >
            <Menu size={20} />
          </button>
          <div className="flex items-center gap-2">
            <OctoLogo size={24} className="text-white" />
            <span className="text-lg font-bold text-white">Octo</span>
          </div>
          <div className="w-8" />
        </header>

        {/* Page content */}
        <main className="flex-1 overflow-y-auto bg-background rounded-tl-xl lg:border-l lg:border-t lg:border-[#222]">
          <Outlet context={{ isExpanded, setIsExpanded }} />
        </main>
      </div>
    </div>
  );
}
