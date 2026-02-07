import { useEffect, useState } from "react";
import { CornerDownLeft } from 'lucide-react';

export default function TypingText({ text, speed = 50 }) {
  const [displayed, setDisplayed] = useState("");

  useEffect(() => {
    let i = 0;
    const interval = setInterval(() => {
      setDisplayed((prev) => prev + text.charAt(i));
      i++;

      if (i === text.length) clearInterval(interval);
    }, speed);

    return () => clearInterval(interval);
  }, [text, speed]);

  return <div className="flex items-center">{displayed}<span className="cursor"></span> <CornerDownLeft size={20} className="text-[#ffffff9e] ml-4" /> </div>;
}
