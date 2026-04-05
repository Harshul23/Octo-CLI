import { TbCopy } from "react-icons/tb";
import { useCopy } from "../../../hooks/useCopy.js";
import React from "react";

const Overview = () => {
  const { copy } = useCopy();
  const text = "brew install harshul23/tap/octo-cli";

  const [isClicked, setIsClicked] = React.useState(false);

  const handleCopyClick = () => {
    copy(text);
    setIsClicked(true);
    setTimeout(() => setIsClicked(false), 150);
  };

  return (
    <div className="bg-[#030303] min-h-screen w-full flex font-sans selection:bg-blue-500/30">
      <div className="h-full mt-24 w-full text-white px-8 lg:px-12 pb-32 flex flex-col gap-16">
        {/* Header Section */}
        <div className="w-full border-b border-white/10 pb-10">
          <h1 className="text-[3.2em] font-bold text-white tracking-tight mb-6 bg-clip-text">
            Octo Documentation
          </h1>
          <p className="text-[1.15em] font-light text-neutral-300 leading-relaxed">
            Octo is an open-source tool that automates local execution of any
            project. It analyzes your codebase, detects the tech stack, and
            prepares the right runtime—whether Docker, Nix, or shell—without
            manual configuration.
            <br />
            <br />
            Built for real-world repositories, Octo understands your project
            structure, generates the required setup, and runs your app the way
            it was meant to be run.
          </p>
        </div>

        {/* Section 1: Octo CLI */}
        <section className="flex flex-col gap-8">
          <div className="flex items-center gap-4 border-l-4 border-blue-500 pl-4">
            <h2 className="text-4xl font-semibold">1. Octo CLI (Terminal)</h2>
            <span className="px-3 py-1 text-xs font-medium bg-blue-500/10 text-blue-400 rounded-full border border-blue-500/20">
              Speed & Control
            </span>
          </div>
          <p className="text-lg font-light text-neutral-400">
            The terminal-first engine of Octo. Designed to be fast and deeply
            integrated with your operating system to orchestrate your "inner
            loop" development seamlessly.
          </p>

          <div className="bg-white/2 border border-white/5 shadow-2xl rounded-2xl p-8 hover:border-white/10 transition-colors">
            <h3 className="text-2xl font-medium mb-4 text-white">
              Installation
            </h3>
            <p className="text-neutral-400 mb-4 font-light">
              Available via Homebrew on macOS and Linux.
            </p>
            <div className="bg-[#0a0a0c] border border-neutral-800 rounded-xl p-4 flex items-center justify-between gap-3 group relative">
              <div className="flex items-center gap-3">
                <span className="text-blue-500 font-mono select-none">$</span>
                <code onClick={handleCopyClick} className="text-neutral-300 font-mono text-sm">
                  brew install harshul23/tap/octo-cli
                </code>
              </div>
              <button
                onClick={handleCopyClick}
                className={`p-2 rounded-lg transition-all duration-200 ${
                  isClicked
                    ? "bg-green-500/20 text-green-400"
                    : "bg-white/5 text-gray-400 hover:text-white hover:bg-white/10"
                }`}
                aria-label="Copy command"
              >
                {isClicked ? (
                  <svg
                    className="w-5 h-5"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                  >
                    <path
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      strokeWidth={2}
                      d="M5 13l4 4L19 7"
                    />
                  </svg>
                ) : (
                  <TbCopy size={20} />
                )}
              </button>
            </div>
          </div>

          <div className="bg-white/2 border border-white/5 shadow-2xl rounded-2xl p-8 hover:border-white/10 transition-colors">
            <h3 className="text-2xl font-medium mb-6 text-white">
              Core Commands
            </h3>
            <div className="grid grid-cols-1 gap-6">
              <div className="flex flex-col gap-2 border-b border-white/5 pb-6">
                <div className="flex items-center gap-3">
                  <code className="text-blue-400 font-mono bg-blue-400/10 px-3 py-1 rounded-md text-sm border border-blue-400/20">
                    octo init
                  </code>
                  <span className="text-sm text-neutral-500">Analyzer</span>
                </div>
                <p className="text-neutral-300 font-light mt-1">
                  Scans your repository, detects frameworks, languages, and
                  dependencies. It automatically generates a{" "}
                  <code className="text-white bg-white/10 px-1 rounded text-sm">
                    blueprint.yaml
                  </code>{" "}
                  configuration tailored to your project.
                </p>
              </div>

              <div className="flex flex-col gap-2 border-b border-white/5 pb-6">
                <div className="flex items-center gap-3">
                  <code className="text-indigo-400 font-mono bg-indigo-400/10 px-3 py-1 rounded-md text-sm border border-indigo-400/20">
                    octo run
                  </code>
                  <span className="text-sm text-neutral-500">Orchestrator</span>
                </div>
                <p className="text-neutral-300 font-light mt-1">
                  The magic command. Reads the blueprint, provisions
                  environments (managing Docker/Nix if required), resolves port
                  conflicts, safely checks dependencies, and starts your
                  application.
                </p>
              </div>

              <div className="flex flex-col gap-2 border-b border-white/5 pb-6">
                <div className="flex items-center gap-3">
                  <code className="text-purple-400 font-mono bg-purple-400/10 px-3 py-1 rounded-md text-sm border border-purple-400/20">
                    octo doctor
                  </code>
                  <span className="text-sm text-neutral-500">Diagnostics</span>
                </div>
                <p className="text-neutral-300 font-light mt-1">
                  Verifies system health. It checks deep into your system's
                  hardware (Thermal Detection) to optimize allocations, ensuring
                  dependencies and Docker Daemons are running optimally.
                </p>
              </div>
            </div>
          </div>
        </section>

        {/* Section 2: Browser Dashboard */}
        <section className="flex flex-col gap-8 mt-8">
          <div className="flex items-center gap-4 border-l-4 border-indigo-500 pl-4">
            <h2 className="text-4xl font-semibold">
              2. Octo Dashboard (Browser)
            </h2>
            <span className="px-3 py-1 text-xs font-medium bg-indigo-500/10 text-indigo-400 rounded-full border border-indigo-500/20">
              Visual Web UI
            </span>
          </div>
          <p className="text-lg font-light text-neutral-400">
            A rich, interactive graphical interface for project orchestration,
            pipeline management, and exploring integrated tools visually.
          </p>

          <div className="w-full h-140 rounded-2xl border">

          </div>

        </section>
      </div>
    </div>
  );
};

export default Overview;
