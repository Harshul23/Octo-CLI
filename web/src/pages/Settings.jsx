import { useState } from "react";
import {
  Palette,
  Lock,
  Settings as SettingsIcon,
  X,
  Github,
  Shield,
  LogOut,
} from "lucide-react";
import { Button } from "../components/ui/Button.jsx";
import { useAuth } from "../contexts/AuthContext.tsx";

export default function Settings() {
  const { user, isAuthenticated, signInWithGitHub, signOut } = useAuth();

  const [activeTab, setActiveTab] = useState("Profile");

  const SETTINGS_TABS = [
    { id: "Profile", icon: Github },
    { id: "CLI Authentication", icon: Shield },
    { id: "Appearance", icon: Palette },
    { id: "Privacy & visibility", icon: Lock },
    { id: "Advanced", icon: SettingsIcon },
  ];

  if (!isAuthenticated) {
    return (
      <div className="flex flex-col items-center justify-center h-full py-20 px-6">
        <p className="text-neutral-400 mb-4">Sign in to access settings.</p>
        <Button onClick={signInWithGitHub}>Sign in with GitHub</Button>
      </div>
    );
  }

  return (
    <div className="h-full w-full flex bg-transparent">
      {/* Sidebar */}
      <div className="w-[260px] border-r border-[#222] hidden md:flex flex-col py-4 shrink-0 overflow-y-auto">
        <div className="px-3 space-y-0.5">
          {SETTINGS_TABS.map((tab) => {
            const isActive = activeTab === tab.id;
            return (
              <button
                key={tab.id}
                onClick={() => setActiveTab(tab.id)}
                className={`w-full flex items-center gap-3 px-3 py-2 rounded-lg text-[13px] font-medium transition-colors ${
                  isActive
                    ? "bg-[#222] text-white"
                    : "text-[#A0A0A0] hover:text-white hover:bg-[#222]/50"
                }`}
              >
                <tab.icon
                  size={16}
                  strokeWidth={2}
                  className={isActive ? "text-white" : "text-[#888]"}
                />
                {tab.id}
              </button>
            );
          })}
        </div>
      </div>

      {/* Content Area */}
      <div className="flex-1 flex flex-col min-w-0 bg-transparent">
        {/* Header */}
        <div className="h-16 px-6 border-b border-[#222] flex items-center justify-between shrink-0">
          <div className="flex items-center gap-2 text-[14px]">
            <span className="text-[#888]">Settings</span>
            <span className="text-[#555]">&gt;</span>
            <span className="text-white font-medium">{activeTab}</span>
          </div>
          <button className="p-1.5 rounded-md border border-[#333] text-[#888] hover:text-white hover:border-[#555] transition-colors">
            <X size={16} />
          </button>
        </div>

        {/* Panel Container */}
        <div className="flex-1 overflow-y-auto p-6 md:p-8 space-y-6">
          {activeTab === "Profile" && (
            <>
              <div className="w-full bg-[#141414] border border-[#222] rounded-xl p-6 min-h-[120px]">
                <h3 className="text-white text-sm font-semibold mb-4">
                  User Profile
                </h3>
                <div className="flex items-center gap-4">
                  {user?.user_metadata?.avatar_url ? (
                    <img
                      src={user.user_metadata.avatar_url}
                      alt="Avatar"
                      className="w-16 h-16 rounded-full ring-2 ring-[#333]"
                    />
                  ) : (
                    <div className="w-16 h-16 rounded-full bg-[#222] border border-[#333] flex items-center justify-center text-white text-xl font-medium">
                      {user?.email?.[0]?.toUpperCase() || "?"}
                    </div>
                  )}
                  <div>
                    <p className="font-medium text-white text-md">
                      {user?.user_metadata?.full_name || "User"}
                    </p>
                    <p className="text-sm text-[#888] mt-1">{user?.email}</p>
                    <p className="text-[12px] text-[#666] mt-2">
                      Signed in via GitHub · Scopes: repo, read:user
                    </p>
                  </div>
                </div>
              </div>
              <div className="w-full bg-[#141414] border border-[#222] rounded-xl p-6 min-h-[160px]">
                <h3 className="text-white text-sm font-semibold mb-4">
                  Account Preferences
                </h3>
                <div className="space-y-4">
                  <div className="flex justify-between items-center pb-4 border-b border-[#222]">
                    <div>
                      <p className="text-sm font-medium text-neutral-200">
                        Log Out
                      </p>
                      <p className="text-xs text-neutral-500">
                        Sign out of your Octo session
                      </p>
                    </div>
                    <Button
                      variant="destructive"
                      size="sm"
                      onClick={signOut}
                      className="bg-red-900/40 hover:bg-red-900/60 text-red-400 border border-red-900/50"
                    >
                      <LogOut size={14} className="mr-2" />
                      Log out
                    </Button>
                  </div>
                </div>
              </div>
            </>
          )}

          {activeTab === "CLI Authentication" && (
            <div className="w-full bg-[#141414] border border-[#222] rounded-xl p-6 min-h-[200px]">
              <h3 className="text-white text-sm font-semibold mb-4">
                CLI Token Configuration
              </h3>
              <p className="text-[13px] text-[#A0A0A0] mb-4 leading-relaxed">
                To authenticate the CLI, generate a token or login securely
                directly from the terminal. Tokens grant root access to your
                configurations.
              </p>
              <div className="flex items-center gap-3 bg-black p-3 rounded-lg border border-[#333] overflow-hidden mb-3">
                <code className="flex-1 text-[13px] text-[#E0E0E0] font-mono truncate">
                  octo login
                </code>
              </div>
              <p className="text-[12px] text-[#777]">
                This will open a browser window to authenticate you.
              </p>
            </div>
          )}

          {/* General Empty State Fillers for the Rest to match screenshot style */}
          {activeTab !== "Profile" && activeTab !== "CLI Authentication" && (
            <div className="w-full bg-[#141414] border border-[#222] rounded-xl p-6 flex flex-col items-center justify-center min-h-[400px]">
              <SettingsIcon size={48} className="text-[#333] mb-4" />
              <h3 className="text-white text-lg font-medium mb-2">
                Coming Soon
              </h3>
              <p className="text-[#888] text-sm text-center max-w-sm">
                These settings are currently in development. Check back later
                for updates to the {activeTab.toLowerCase()} options.
              </p>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
