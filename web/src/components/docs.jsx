import Sidebar from './sidebar'

const Docs = () => {
  return (
    <div className='h-screen w-full text-white pt-2 bg-black flex'>
        <Sidebar />
        <div className='pl-12 py-8'>
          <span className='text-[2.5em] font-medium'>Octo  CLI  documentation</span>
        </div>
    </div>
  )
}

export default Docs