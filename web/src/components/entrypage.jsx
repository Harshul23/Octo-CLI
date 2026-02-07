import TypingText from './TypingText'

const Entrypage = () => {
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
