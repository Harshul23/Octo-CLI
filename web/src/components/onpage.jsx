import React from 'react'

const Onpage = () => {
  return (
    <div className='text-white flex gap-2 items-start px-24 fixed h-9/10 w-3/10 py-40 flex-col bg-black'>
        <span className='text-xl font-medium'>On this page</span>
        <ul className='pl-4 text-neutral-500'>
            <li>Get Started</li>
            <li>How it works</li>
            <li>Configuration</li>
            <li>Thermal Management</li>
            <li>FAQ</li>
        </ul>
    </div>
  )
}

export default Onpage