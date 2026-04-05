import { TbCopy } from "react-icons/tb";
import { useCopy } from "../../hooks/useCopy.js";
import React from "react";
import { useNavigate } from "react-router-dom";
import { useAuth } from "../../contexts/AuthContext";
import ScrollZoomVideo from "../../components/landing/video/videoframe.jsx";
import video from "../../assets/octo-final.mp4";
import FadeText from "../../components/landing/ui/FadeText.jsx";

const Homepage = () => {
  const { copy } = useCopy();
  const navigate = useNavigate();
  const { isAuthenticated, signInWithGitHub } = useAuth();
  const text = "brew install harshul23/tap/octo-cli";

  const [isClicked, setIsClicked] = React.useState(false);

  const handleCopyClick = () => {
    copy(text);
    setIsClicked(true);
    setTimeout(() => setIsClicked(false), 150);
  };

  return (
    <div className="flex flex-col w-full bg-black">
      <div className="flex flex-col items-center justify-center w-full gap-10 overflow-scroll min-h-8/10 mt-45">
        <div className="flex flex-col items-center justify-center w-full h-full gap-6">
          <p className="text-7xl font-normal py-4 leading-20 px-[3em] text-white text-center h-full w-full">
            <FadeText text="Now local execution is automated from detection to deployment" />
          </p>
          <p className="text-3xl font-light py-4 px-[8em] text-[#ffffffb5] text-center h-full w-full">
            A single command that understands your project, prepares everything
            it needs, and runs it the way it was meant to.
          </p>

          <div className="flex flex-col items-center justify-center w-full gap-10 mt-4 md:flex-row">
            <button
              onClick={() => {
                if (isAuthenticated) {
                  window.open("/dashboard", "_blank");
                } else {
                  const apiUrl = import.meta.env.VITE_API_URL || "";
                  window.open(`${apiUrl}/api/auth/github`, "_blank");
                }
              }}
              className="group relative px-6 py-3 bg-white text-black font-semibold rounded-xl hover:bg-neutral-200 transition-all duration-300 flex items-center gap-2 shadow-[0_0_20px_rgba(255,255,255,0.15)] hover:shadow-[0_0_30px_rgba(255,255,255,0.3)]"
            >
              Configure in browser
              <svg
                className="w-4 h-4 transition-transform group-hover:translate-x-1"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M14 5l7 7m0 0l-7 7m7-7H3"
                />
              </svg>
            </button>

            <div className="relative flex flex-col group">
              <div className="flex items-center gap-3"></div>
              <div className="flex items-center bg-[#09090b] border border-[#27272a] group-hover:border-blue-500/50 transition-colors duration-300 rounded-xl overflow-hidden pl-4 pr-1.5 py-1.5 shadow-[0_0_20px_rgba(0,0,0,0.5)] group-hover:shadow-[0_0_25px_rgba(59,130,246,0.15)]">
                <span className="mr-3 font-mono text-sm text-blue-500">$</span>
                <code onClick={handleCopyClick} className="mr-4 font-mono text-sm text-gray-300 whitespace-nowrap">
                  brew install harshul23/tap/octo-cli
                </code>
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
          </div>
        </div>
        <ScrollZoomVideo src={video} />
      </div>
    </div>
  );
};

export default Homepage;
