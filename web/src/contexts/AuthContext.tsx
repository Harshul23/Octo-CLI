import React, { createContext, useContext, useState, useEffect } from "react";
import api from "../services/api";

interface User {
  id: string;
  email: string;
  name: string;
  avatar_url: string;
  github_username: string;
}

interface AuthContextType {
  user: User | null;
  isAuthenticated: boolean;
  isLoading: boolean;
  signInWithGitHub: () => void;
  signOut: () => Promise<void>;
}

const AuthContext = createContext<AuthContextType | undefined>(undefined);

const API_URL = import.meta.env.VITE_API_URL || "";

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [isLoading, setIsLoading] = useState(true);

  // Check for existing session on mount
  useEffect(() => {
    checkSession();
  }, []);

  const checkSession = async () => {
    try {
      // Parse token from URL if present
      const params = new URLSearchParams(window.location.search);
      let sessionToken = params.get("token");

      if (sessionToken) {
        localStorage.setItem("octo_session_token", sessionToken);
        // Clean URL to not expose token
        window.history.replaceState({}, document.title, window.location.pathname);
      } else {
        sessionToken = localStorage.getItem("octo_session_token");
      }

      const headers: Record<string, string> = {};
      if (sessionToken) {
        api.setToken(sessionToken);
        headers["Authorization"] = `Bearer ${sessionToken}`;
      }

      const response = await fetch(`${API_URL}/api/auth/me`, {
        headers,
        credentials: "include",
      });

      if (response.ok) {
        const userData = await response.json();
        setUser(userData);
      } else {
        setUser(null);
        localStorage.removeItem("octo_session_token");
        api.setToken(null);
      }
    } catch (error) {
      console.error("Failed to check session:", error);
      setUser(null);
      localStorage.removeItem("octo_session_token");
      api.setToken(null);
    } finally {
      setIsLoading(false);
    }
  };

  const signInWithGitHub = () => {
    window.location.href = `${API_URL}/api/auth/github`;
  };

  const signOut = async () => {
    try {
      const sessionToken = localStorage.getItem("octo_session_token");
      const headers: Record<string, string> = {};
      if (sessionToken) {
        headers["Authorization"] = `Bearer ${sessionToken}`;
      }

      await fetch(`${API_URL}/api/auth/logout`, {
        method: "POST",
        headers,
        credentials: "include",
      });
    } catch (error) {
      console.error("Failed to sign out:", error);
    } finally {
      setUser(null);
      localStorage.removeItem("octo_session_token");
      api.setToken(null);
      window.location.href = "/";
    }
  };

  const value: AuthContextType = {
    user,
    isAuthenticated: !!user,
    isLoading,
    signInWithGitHub,
    signOut,
  };

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth() {
  const context = useContext(AuthContext);
  if (context === undefined) {
    throw new Error("useAuth must be used within an AuthProvider");
  }
  return context;
}
