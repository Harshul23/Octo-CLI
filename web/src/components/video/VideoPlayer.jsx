import { useRef, useState } from "react";

export default function VideoPlayer({ src, poster }) {
  const videoRef = useRef(null);
  const containerRef = useRef(null);

  const [isPlaying, setIsPlaying] = useState(false);
  const [hovered, setHovered] = useState(false);
  const [cursorPos, setCursorPos] = useState({ x: 0, y: 0 });

  const togglePlay = () => {
    if (!videoRef.current) return;

    if (isPlaying) {
      videoRef.current.pause();
    } else {
      videoRef.current.play();
    }

    setIsPlaying(!isPlaying);
  };

  const handleMouseMove = (e) => {
    const rect = containerRef.current.getBoundingClientRect();
    setCursorPos({
      x: e.clientX - rect.left,
      y: e.clientY - rect.top,
    });
  };

  return (
    <div
      ref={containerRef}
      className="relative w-full max-w-3xl mx-auto group"
      onMouseEnter={() => setHovered(true)}
      onMouseLeave={() => setHovered(false)}
      onMouseMove={handleMouseMove}
      onClick={togglePlay}
    >
      {/* Video */}
      <video
        ref={videoRef}
        src={src}
        poster={poster}
        className={`w-full rounded-2xl shadow-xl ${
          hovered ? "cursor-none" : ""
        }`}
      />

      {/* Gradient overlay */}
      <div className="absolute inset-0 rounded-2xl bg-gradient-to-t from-black/30 to-transparent pointer-events-none" />

      {/* Custom Cursor */}
      {hovered && (
        <div
          className="absolute pointer-events-none transition-transform duration-75"
          style={{
            left: cursorPos.x,
            top: cursorPos.y,
            transform: "translate(-50%, -50%)",
          }}
        >
          <div className="bg-white/90 backdrop-blur-md shadow-lg rounded-full p-4">
            {isPlaying ? <PauseIcon /> : <PlayIcon />}
          </div>
        </div>
      )}
    </div>
  );
}

function PlayIcon() {
  return (
    <svg width="28" height="28" viewBox="0 0 24 24" fill="black">
      <path d="M8 5v14l11-7z" />
    </svg>
  );
}

function PauseIcon() {
  return (
    <svg width="28" height="28" viewBox="0 0 24 24" fill="black">
      <path d="M6 5h4v14H6zm8 0h4v14h-4z" />
    </svg>
  );
}