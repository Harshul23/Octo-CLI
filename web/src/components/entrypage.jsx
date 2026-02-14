import TypingText from './TypingText'
import { useEffect } from "react";
import { useNavigate } from "react-router-dom";

const Entrypage = () => {

  const navigate = useNavigate();

  useEffect(() => {
    const timer = setTimeout(() => {
      navigate("/home");
    }, 1200);

    return () => clearTimeout(timer);
  }, [navigate]);


  return (
    <div className='flex bg-black justify-center items-center h-screen w-full'>
      <div className='text-white text-2xl h-full w-full mr-[4em] flex items-center justify-center gap-4'>
        <span className='text-2xl text-[#ffffff9e]'>developers-terminal ~ % </span>
        <TypingText text="oocto run"/>
      </div>
    </div>
  )
}

export default Entrypage
