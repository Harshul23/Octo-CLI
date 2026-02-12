import { motion, useMotionValue, useSpring } from "framer-motion";
import { useEffect, useRef, useState } from "react";

export default function ScrollZoomVideo({ src }) {
  const sectionRef = useRef(null);
  const videoRef = useRef(null);

  const rawScale = useMotionValue(0.8);
  const scale = useSpring(rawScale, {
    stiffness: 80,
    damping: 20,
    mass: 0.5,
  });

  const [locked, setLocked] = useState(false);
  const [isInViewport, setIsInViewport] = useState(false);
  const [hovered, setHovered] = useState(false);
  const [isPlaying, setIsPlaying] = useState(true);

  const MIN = 0.6;
  const MAX = 0.9;
  const UNLOCK_THRESHOLD = 100;
  const extraScrollRef = useRef(0);

  // Visibility detection
  useEffect(() => {
    const observer = new IntersectionObserver(
      ([entry]) => {
        setIsInViewport(entry.isIntersecting);

        if (!entry.isIntersecting) {
          setLocked(false);
          document.body.style.overflow = "auto";
        }
      },
      { threshold: 0.8 }
    );

    if (sectionRef.current) {
      observer.observe(sectionRef.current);
    }

    return () => {
      observer.disconnect();
      document.body.style.overflow = "auto";
    };
  }, []);

  // Scroll zoom logic
  useEffect(() => {
    const handleWheel = (e) => {
      if (!isInViewport) return;

      const current = rawScale.get();
      const delta = e.deltaY * 0.0008;
      let next = current + delta;

      const atMin = current <= MIN;
      const atMax = current >= MAX;
      const scrollingUp = e.deltaY < 0;
      const scrollingDown = e.deltaY > 0;

      const canZoomOut = scrollingUp && !atMin;
      const canZoomIn = scrollingDown && !atMax;
      const canZoom = canZoomOut || canZoomIn;

      if (canZoom) {
        e.preventDefault();

        if (!locked) {
          setLocked(true);
          document.body.style.overflow = "hidden";
        }

        extraScrollRef.current = 0;
        rawScale.set(Math.max(MIN, Math.min(MAX, next)));
      } else if (locked) {
        e.preventDefault();

        extraScrollRef.current += Math.abs(e.deltaY);

        if (extraScrollRef.current > UNLOCK_THRESHOLD) {
          document.body.style.overflow = "auto";
          setLocked(false);
          extraScrollRef.current = 0;
        }
      }
    };

    window.addEventListener("wheel", handleWheel, { passive: false });

    return () => {
      window.removeEventListener("wheel", handleWheel);
    };
  }, [isInViewport, locked, rawScale]);

  // Cursor follow
  const handleMouseMove = (e) => {
    const cursor = document.getElementById("custom-cursor");
    if (!cursor) return;

    cursor.style.left = e.clientX + "px";
    cursor.style.top = e.clientY + "px";
  };

  // Toggle play
  const togglePlay = () => {
    if (!videoRef.current) return;

    if (videoRef.current.paused) {
      videoRef.current.play();
      setIsPlaying(true);
    } else {
      videoRef.current.pause();
      setIsPlaying(false);
    }
  };

  return (
    <section
      ref={sectionRef}
      className="h-screen flex items-center justify-center bg-black relative"
    >
      <motion.div
        style={{ scale }}
        className={`w-[90%] max-w-5xl border-2 border-[white] rounded-2xl overflow-hidden shadow-2xl ${
          hovered ? "cursor-none" : ""
        }`}
        onClick={togglePlay}
        onMouseMove={handleMouseMove}
        onMouseEnter={() => setHovered(true)}
        onMouseLeave={() => setHovered(false)}
      >
        <video
          ref={videoRef}
          src={src}
          className="w-full h-full object-cover"
          autoPlay
          playsInline
          preload="metadata"
          muted
          loop
        />
      </motion.div>

      {hovered && (
        <div
          id="custom-cursor"
          className="fixed pointer-events-none z-50 transition-all duration-150 ease-out"
          style={{ transform: "translate(-50%, -50%)" }}
        >
          <div className="bg-white/90 backdrop-blur-md shadow-lg rounded-full p-4">
            {isPlaying ? (
              <svg width="28" height="28" viewBox="0 0 24 24" fill="black">
                <path d="M6 5h4v14H6zm8 0h4v14h-4z" />
              </svg>
            ) : (
              <svg width="28" height="28" viewBox="0 0 24 24" fill="black">
                <path d="M8 5v14l11-7z" />
              </svg>
            )}
          </div>
        </div>
      )}
    </section>
  );
}