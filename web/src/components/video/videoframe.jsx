import { motion, useMotionValue, useSpring } from "framer-motion";
import { useEffect, useRef, useState } from "react";

export default function ScrollZoomVideo({ src }) {
  const sectionRef = useRef(null);

  const rawScale = useMotionValue(0.8);

  // Smooth physics
  const scale = useSpring(rawScale, {
    stiffness: 120,
    damping: 20,
    mass: 0.5,
  });

  const [locked, setLocked] = useState(false);

  const MIN = 0.8;
  const MAX = 1;

  // Detect section visibility
  useEffect(() => {
    const observer = new IntersectionObserver(
      ([entry]) => {
        if (entry.isIntersecting) {
          setLocked(true);
          document.body.style.overflow = "hidden";
        }
      },
      { threshold: 0.6 }
    );

    observer.observe(sectionRef.current);
    return () => observer.disconnect();
  }, []);

  // Smooth wheel control
  useEffect(() => {
    if (!locked) return;

    const handleWheel = (e) => {
      e.preventDefault();

      const current = rawScale.get();

      // Scale change proportional to scroll speed
      const delta = e.deltaY * 0.0008;

      let next = current + delta;

      // Clamp
      next = Math.max(MIN, Math.min(MAX, next));

      rawScale.set(next);

      // Unlock when limits reached AND user continues scrolling
      if ((next === MAX && e.deltaY > 0) || (next === MIN && e.deltaY < 0)) {
        document.body.style.overflow = "auto";
        setLocked(false);
      }
    };

    window.addEventListener("wheel", handleWheel, { passive: false });

    return () => {
      window.removeEventListener("wheel", handleWheel);
      document.body.style.overflow = "auto";
    };
  }, [locked, rawScale]);

  return (
    <section
      ref={sectionRef}
      className="h-screen flex items-center justify-center bg-black"
    >
      <motion.div
        style={{ scale }}
        className="w-[80%] max-w-5xl rounded-2xl overflow-hidden shadow-2xl"
      >
        <video
          src={src}
          className="w-full h-full object-cover"
          autoPlay
          muted
          loop
        />
      </motion.div>
    </section>
  );
}