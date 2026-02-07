import Homepage from './components/homepage.jsx'
import Entrypage from './components/entrypage.jsx'
import { useEffect, useState } from "react";

function App() {
  const [showEntry, setShowEntry] = useState(true);

  useEffect(() => {
    const timer = setTimeout(() => {
      setShowEntry(false);
    }, 1200);

    return () => clearTimeout(timer);
  }, []);

  return showEntry ? <Entrypage /> : <Homepage />;
}

export default App
