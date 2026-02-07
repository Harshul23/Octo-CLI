import { useEffect, useState } from "react";

export default function FadeText({ text, delay = 0 }) {
  const [show, setShow] = useState(false);

  useEffect(() => {
    const t = setTimeout(() => setShow(true), delay);
    return () => clearTimeout(t);
  }, [delay]);

  return (
    <p
      className={`transition-all duration-1000 ease-out
        ${show ? "opacity-100 translate-y-0" : "opacity-0 translate-y-3"}
      `}
    >
      {text}
    </p>
  );
}