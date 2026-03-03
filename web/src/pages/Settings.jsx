import { useAuth } from "../contexts/AuthContext.jsx";
import { Github, Shield, Palette } from "lucide-react";
import { Button } from "../components/ui/Button.jsx";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "../components/ui/Card.jsx";

export default function Settings() {
  const { user, isAuthenticated, signInWithGitHub } = useAuth();

  if (!isAuthenticated) {
    return (
      <div className="flex flex-col items-center justify-center h-full py-20 px-6">
        <p className="text-muted-foreground mb-4">
          Sign in to access settings.
        </p>
        <Button onClick={signInWithGitHub}>Sign in with GitHub</Button>
      </div>
    );
  }

  return (
    <div className="p-6 lg:p-8 max-w-3xl mx-auto">
      <h1 className="text-2xl font-bold mb-8">Settings</h1>

      {/* Profile */}
      <Card className="mb-6">
        <CardHeader>
          <CardTitle className="text-base flex items-center gap-2">
            <Github size={18} />
            Profile
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div className="flex items-center gap-4">
            {user?.user_metadata?.avatar_url && (
              <img
                src={user.user_metadata.avatar_url}
                alt="Avatar"
                className="w-14 h-14 rounded-full ring-2 ring-border"
              />
            )}
            <div>
              <p className="font-medium">
                {user?.user_metadata?.full_name || "User"}
              </p>
              <p className="text-sm text-muted-foreground">{user?.email}</p>
              <p className="text-xs text-muted-foreground/60 mt-1">
                Signed in via GitHub · Scopes: repo, read:user
              </p>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* API Token */}
      <Card className="mb-6">
        <CardHeader>
          <CardTitle className="text-base flex items-center gap-2">
            <Shield size={18} />
            CLI Authentication
          </CardTitle>
        </CardHeader>
        <CardContent>
          <p className="text-sm text-muted-foreground mb-3">
            Use this token with{" "}
            <code className="text-foreground font-mono text-xs">
              octo publish
            </code>{" "}
            to push configs from the CLI.
          </p>
          <div className="flex items-center gap-3">
            <code className="flex-1 px-3 py-2 bg-muted border border-border rounded-lg text-sm text-muted-foreground font-mono truncate">
              {`OCTO_TOKEN=<your-supabase-access-token>`}
            </code>
          </div>
          <p className="text-xs text-muted-foreground/60 mt-2">
            Get your token from the Supabase dashboard or use{" "}
            <code className="font-mono">supabase auth token</code>.
          </p>
        </CardContent>
      </Card>

      {/* Preferences */}
      <Card>
        <CardHeader>
          <CardTitle className="text-base flex items-center gap-2">
            <Palette size={18} />
            Preferences
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div className="space-y-4">
            <div className="flex items-center justify-between">
              <div>
                <p className="text-sm font-medium">Default visibility</p>
                <p className="text-xs text-muted-foreground">
                  New projects are public by default
                </p>
              </div>
              <Button variant="outline" size="sm">
                Public
              </Button>
            </div>
            <div className="flex items-center justify-between">
              <div>
                <p className="text-sm font-medium">Thermal mode</p>
                <p className="text-xs text-muted-foreground">
                  Default thermal config for new blueprints
                </p>
              </div>
              <Button variant="outline" size="sm">
                Auto
              </Button>
            </div>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
