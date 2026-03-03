import { createContext, useContext, useState, useEffect } from "react";
import { supabase } from "../lib/supabase.js";
import api from "../services/api.js";

const AuthContext = createContext(null);

export function AuthProvider({ children }) {
  const [user, setUser] = useState(null);
  const [session, setSession] = useState(null);
  const [providerToken, setProviderToken] = useState(
    () => localStorage.getItem("gh_provider_token") || null,
  );
  const [loading, setLoading] = useState(true);

  const persistProviderToken = async (token) => {
    setProviderToken(token);
    localStorage.setItem("gh_provider_token", token);
    // Persist to Supabase user_tokens table (best-effort)
    try {
      await api.upsertUserToken("github", token, "repo read:user");
    } catch {
      // Silently fail — table may not exist yet
    }
  };

  useEffect(() => {
    // Get initial session
    supabase.auth.getSession().then(({ data: { session } }) => {
      setSession(session);
      setUser(session?.user ?? null);
      if (session?.provider_token) {
        persistProviderToken(session.provider_token);
      }
      setLoading(false);
    });

    // Listen for auth changes
    const {
      data: { subscription },
    } = supabase.auth.onAuthStateChange((_event, session) => {
      setSession(session);
      setUser(session?.user ?? null);
      if (session?.provider_token) {
        persistProviderToken(session.provider_token);
      }
      if (_event === "SIGNED_OUT") {
        setProviderToken(null);
        localStorage.removeItem("gh_provider_token");
      }
      setLoading(false);
    });

    return () => subscription.unsubscribe();
  }, []);

  const signInWithGitHub = async () => {
    const { error } = await supabase.auth.signInWithOAuth({
      provider: "github",
      options: {
        redirectTo: `${window.location.origin}/auth/callback`,
        scopes: "repo read:user",
      },
    });
    if (error) console.error("Auth error:", error);
  };

  const signOut = async () => {
    await supabase.auth.signOut();
    setUser(null);
    setSession(null);
    setProviderToken(null);
    localStorage.removeItem("gh_provider_token");
  };

  const value = {
    user,
    session,
    loading,
    providerToken,
    signInWithGitHub,
    signOut,
    isAuthenticated: !!session,
    accessToken: session?.access_token,
  };

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth() {
  const context = useContext(AuthContext);
  if (!context) {
    throw new Error("useAuth must be used within an AuthProvider");
  }
  return context;
}
