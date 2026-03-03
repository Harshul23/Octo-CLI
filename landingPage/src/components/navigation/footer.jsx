import React from 'react'

const Footer = () => {
  return (
    <div className='h-1/10 z-10 bg-black mt-10 py-4 px-4 w-full gap-10 flex items-center'>
        <div>
            <p className='text-3xl font-light text-[#ffffffe1]'><span className='font-bold mr-1.5'>Octo</span> for Developers</p>
        </div>
        <div className='flex items-center gap-4 mt-2'>
            <p className='text-sm text-[#ffffffbb] hover:underline'>Terms</p>
            <p className='bg-[#ffffffb0] h-4 w-0.5'></p>
            <p className='text-sm text-[#ffffffbb] hover:underline'>Privacy</p>
        </div>
    </div>
  )
}

export default Footer