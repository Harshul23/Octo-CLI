import { useEffect } from "react";
import { useNavigate } from "react-router-dom";
import { supabase } from "../lib/supabase.js";

export default function AuthCallback() {
  const navigate = useNavigate();

  useEffect(() => {
    // Supabase handles the OAuth callback automatically via the URL hash
    // We just need to wait for the session to be set and redirect
    const handleAuth = async () => {
      const {
        data: { session },
        error,
      } = await supabase.auth.getSession();

      if (error) {
        console.error("Auth callback error:", error);
        navigate("/");
        return;
      }

      if (session) {
        navigate("/dashboard");
      } else {
        // Wait a moment for Supabase to process the callback
        setTimeout(async () => {
          const {
            data: { session: retrySession },
          } = await supabase.auth.getSession();
          if (retrySession) {
            navigate("/dashboard");
          } else {
            navigate("/");
          }
        }, 1000);
      }
    };

    handleAuth();
  }, [navigate]);

  return (
    <div className="min-h-screen bg-black flex items-center justify-center">
      <div className="text-center">
        <div className="w-12 h-12 border-2 border-emerald-500 border-t-transparent rounded-full animate-spin mx-auto mb-4" />
        <p className="text-neutral-400 text-sm">Signing you in...</p>
      </div>
    </div>
  );
}
