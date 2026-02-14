import { ChevronDown } from 'lucide-react';
import { useState } from 'react';

const Sidebar = () => {
  const [dropdown, setdropdown] = useState('show')

  const toggle = () => {
    dropdown == 'show' ? setdropdown('hide') : setdropdown('show')
  }

  return (
    <div className='flex flex-col h-screen border-r-2 border-neutral-700 w-2/10 px-4 py-4 bg-[black]'>
      <div className='flex h-1/12 w-full justify-between items-center'>
        <span className='text-lg'>Get Started</span>
        <p className='h-4 w-4 flex justify-center items-center rounded-full'><ChevronDown onClick={toggle} className={`mt-2 transition-all ease-in ${dropdown == 'hide' ? "-rotate-90" : "rotate-0"}`} size={20} /></p>
      </div>
      <div className='w-full border-l border-neutral-500 px-4 ml-2'>
        <ul className={`flex flex-col gap-2 transition-all ease-in ${dropdown == 'hide' ? "h-0 overflow-hidden" : "h-auto"}`}>
          <li className='text-sm hover:text-white text-neutral-400'>Installation</li>
          <li className='text-sm hover:text-white text-neutral-400'>Usage</li>
          <li className='text-sm hover:text-white text-neutral-400'>Configuration</li>
        </ul>
      </div>
    </div>
  )
}

export default Sidebar