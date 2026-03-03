import { TbCopy } from "react-icons/tb";
import { useCopy } from "../hooks/useCopy.js";
import React from 'react';
import ScrollZoomVideo from "../components/video/videoframe.jsx";
import video from "../assets/octo-final.mp4"
import FadeText from "../components/ui/FadeText.jsx";

const Homepage = () => {

  const { copy } = useCopy();
    const text = "brew install harshul23/tap/octo-cli";
  
    const [isClicked, setIsClicked] = React.useState(false);
  
    const handleCopyClick = () => {
      copy(text);
      setIsClicked(true);
      setTimeout(() => setIsClicked(false), 150);
    };

  return (
    <div className='flex  flex-col w-full bg-black'>   
      <div className='min-h-8/10 mt-45 gap-10 overflow-scroll w-full flex flex-col items-center justify-center'>
              <div className='flex-col gap-6 flex items-center justify-center w-full h-full'>
                  <p className='text-7xl font-normal py-4 leading-20 px-[3em] text-white text-center h-full w-full'><FadeText text="Now local execution is automated from detection to deployment" /></p>
                  <p className='text-3xl font-light py-4 px-[8em] text-[#ffffffb5] text-center h-full w-full'>A single command that understands your project, prepares everything it needs, and runs it the way it was meant to.</p>
                  <div className='text-sm bg-[#17131d] text-white px-4 py-2 rounded-xl border-2 border-blue-400'>
                      <code>
                        <pre className='flex items-center gap-4'>
                          $ brew install harshul23/tap/octo-cli       
                          <TbCopy
                            onClick={handleCopyClick}
                            size={22}
                            style={{
                              transition: 'transform 0.15s',
                              transform: isClicked ? 'scale(0.85)' : 'scale(1)'
                            }}
                          />
                        </pre>
                      </code>
                  </div>
                  <p className='text-sm text-[#d786ff]'><pre>More install options</pre></p>
              </div>
              <ScrollZoomVideo src={video} />
          </div>
    </div>
  )
}

export default Homepage
