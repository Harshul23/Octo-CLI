import { useNavigate } from "react-router-dom";
import { useAuth } from "../contexts/AuthContext.jsx";
import { motion } from "framer-motion";
import {
  Github,
  Zap,
  Globe,
  Terminal,
  ArrowRight,
  Boxes,
  Shield,
} from "lucide-react";
import { Button } from "../components/ui/Button.jsx";
import { Card } from "../components/ui/Card.jsx";
import { Badge } from "../components/ui/Badge.jsx";
import OctoLogo from "../components/ui/OctoLogo.jsx";

const features = [
  {
    icon: Zap,
    title: "Instant Analysis",
    desc: "Paste a GitHub URL and get a complete project blueprint in seconds.",
  },
  {
    icon: Terminal,
    title: "One-Command Run",
    desc: "Other developers run your project with a single `octo run` command.",
  },
  {
    icon: Globe,
    title: "Share Configs",
    desc: "Publish your .octo.yaml and make any project instantly runnable.",
  },
  {
    icon: Boxes,
    title: "Multi-Runtime",
    desc: "Supports Node, Python, Go, Java, Rust, Ruby — and monorepos.",
  },
  {
    icon: Shield,
    title: "Secrets Safe",
    desc: "Environment variables are templated, never exposed in shared configs.",
  },
  {
    icon: Github,
    title: "GitHub Native",
    desc: "Import directly from your repos. OAuth integration built in.",
  },
];

export default function Landing() {
  const { signInWithGitHub, isAuthenticated } = useAuth();
  const navigate = useNavigate();

  const handleGetStarted = () => {
    if (isAuthenticated) {
      navigate("/dashboard");
    } else {
      signInWithGitHub();
    }
  };

  return (
    <div className="min-h-screen bg-background text-foreground">
      {/* Navbar */}
      <nav className="fixed top-0 left-0 right-0 z-50 border-b border-border/50 bg-background/80 backdrop-blur-xl">
        <div className="max-w-6xl mx-auto flex items-center justify-between px-6 py-4">
          <div className="flex items-center gap-2">
            <OctoLogo size={32} />
            <span className="text-xl font-bold">Octo</span>
          </div>
          <div className="flex items-center gap-4">
            <a
              href="/explore"
              className="text-sm text-muted-foreground hover:text-foreground transition-colors"
            >
              Explore
            </a>
            <a
              href="https://github.com/Harshul23/Octo-CLI"
              target="_blank"
              rel="noopener noreferrer"
              className="text-sm text-muted-foreground hover:text-foreground transition-colors"
            >
              GitHub
            </a>
            <Button onClick={handleGetStarted} size="sm">
              {isAuthenticated ? "Dashboard" : "Get Started"}
            </Button>
          </div>
        </div>
      </nav>

      {/* Hero */}
      <section className="pt-32 pb-20 px-6">
        <div className="max-w-4xl mx-auto text-center">
          <motion.div
            initial={{ opacity: 0, y: 20 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.6 }}
          >
            <Badge variant="outline" className="mb-6 gap-1.5">
              <Zap size={12} />
              Zero-friction local deployment
            </Badge>

            <h1 className="text-5xl sm:text-7xl font-bold tracking-tight leading-[1.1] mb-6">
              <span className="text-foreground">Octo-fy your</span>
              <br />
              <span className="text-muted-foreground">repositories</span>
            </h1>

            <p className="text-lg sm:text-xl text-muted-foreground max-w-2xl mx-auto mb-10 leading-relaxed">
              Analyze any GitHub repo, generate a deployment config, and let any
              developer run it with a single command. No more README decay.
            </p>

            <div className="flex flex-col sm:flex-row items-center justify-center gap-4">
              <Button
                onClick={handleGetStarted}
                size="lg"
                className="gap-2 group"
              >
                <Github size={18} />
                {isAuthenticated ? "Open Dashboard" : "Sign in with GitHub"}
                <ArrowRight
                  size={16}
                  className="group-hover:translate-x-0.5 transition-transform"
                />
              </Button>
              <Button variant="outline" size="lg" asChild className="gap-2">
                <a href="/explore">
                  <Boxes size={18} />
                  Explore Projects
                </a>
              </Button>
            </div>

            {/* Scope notice */}
            <p className="mt-6 text-xs text-muted-foreground/60">
              OAuth grants read/write access to your repositories for analysis.
              We never push or modify your code.
            </p>
          </motion.div>

          {/* Terminal preview */}
          <motion.div
            initial={{ opacity: 0, y: 40 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.8, delay: 0.3 }}
            className="mt-16 max-w-2xl mx-auto"
          >
            <Card className="overflow-hidden shadow-2xl">
              <div className="flex items-center gap-2 px-4 py-3 border-b border-border">
                <div className="w-3 h-3 rounded-full bg-muted-foreground/30" />
                <div className="w-3 h-3 rounded-full bg-muted-foreground/30" />
                <div className="w-3 h-3 rounded-full bg-muted-foreground/30" />
                <span className="ml-2 text-xs text-muted-foreground">
                  terminal
                </span>
              </div>
              <div className="p-5 font-mono text-sm space-y-2">
                <div>
                  <span className="text-foreground">$</span>{" "}
                  <span className="text-foreground font-medium">octo init</span>
                </div>
                <div className="text-muted-foreground">
                  🔍 Analyzing codebase...
                </div>
                <div className="text-muted-foreground">
                  📦 Detected: Node.js (v20.x) + pnpm
                </div>
                <div className="text-muted-foreground">
                  🔑 Found 3 environment variables
                </div>
                <div className="text-foreground">✅ Generated .octo.yaml</div>
                <div className="mt-3">
                  <span className="text-foreground">$</span>{" "}
                  <span className="text-foreground font-medium">octo run</span>
                </div>
                <div className="text-muted-foreground">
                  🚀 Starting application on http://localhost:3000
                </div>
              </div>
            </Card>
          </motion.div>
        </div>
      </section>

      {/* Features */}
      <section className="py-20 px-6 border-t border-border/50">
        <div className="max-w-6xl mx-auto">
          <div className="text-center mb-16">
            <h2 className="text-3xl font-bold mb-4">Everything you need</h2>
            <p className="text-muted-foreground max-w-xl mx-auto">
              From analysis to deployment — Octo handles the entire setup
              pipeline.
            </p>
          </div>
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-6">
            {features.map((feat, i) => (
              <motion.div
                key={feat.title}
                initial={{ opacity: 0, y: 20 }}
                whileInView={{ opacity: 1, y: 0 }}
                viewport={{ once: true }}
                transition={{ delay: i * 0.1 }}
              >
                <Card className="p-6 h-full hover:border-foreground/20 transition-colors">
                  <div className="w-10 h-10 rounded-lg bg-accent flex items-center justify-center mb-4">
                    <feat.icon size={20} className="text-foreground" />
                  </div>
                  <h3 className="text-base font-semibold mb-2">{feat.title}</h3>
                  <p className="text-sm text-muted-foreground leading-relaxed">
                    {feat.desc}
                  </p>
                </Card>
              </motion.div>
            ))}
          </div>
        </div>
      </section>

      {/* CTA */}
      <section className="py-20 px-6 border-t border-border/50">
        <div className="max-w-2xl mx-auto text-center">
          <h2 className="text-3xl font-bold mb-4">Ready to Octo-fy?</h2>
          <p className="text-muted-foreground mb-8">
            Start by analyzing a GitHub repository or install the CLI locally.
          </p>
          <div className="flex flex-col sm:flex-row gap-4 justify-center">
            <Button onClick={handleGetStarted}>Get Started Free</Button>
            <Card className="flex items-center gap-2 px-4 py-3 font-mono text-sm text-muted-foreground">
              <span className="text-foreground">$</span> brew install octo-cli
            </Card>
          </div>
        </div>
      </section>

      {/* Footer */}
      <footer className="py-8 px-6 border-t border-border/50">
        <div className="max-w-6xl mx-auto flex items-center justify-between text-xs text-muted-foreground">
          <span>© 2026 Octo CLI. MIT License.</span>
          <div className="flex items-center gap-4">
            <a
              href="https://github.com/Harshul23/Octo-CLI"
              className="hover:text-foreground transition-colors"
            >
              GitHub
            </a>
            <a href="/docs" className="hover:text-foreground transition-colors">
              Docs
            </a>
          </div>
        </div>
      </footer>
    </div>
  );
}
