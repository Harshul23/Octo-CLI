import FadeText from './FadeText'

const Heropage = () => {
  return (
    <div className='h-[90vh] w-full flex flex-col items-center justify-center'>
        <div className='flex-col gap-6 flex items-center justify-center w-full h-full'>
            <p className='text-6xl font-medium px-[3em] text-white text-center w-full'><FadeText text="Now local execution is automated from detection to deployment" /></p>
            <p className='text-2xl font-normal px-[9em] text-[#ffffffb5] text-center w-full'>A single command that understands your project, prepares everything it needs, and runs it the way it was meant to.</p>
            <div className='text-sm bg-[#17131d] text-white px-4 py-2 rounded-xl border-2 border-[#ac87eb]'>
                <code><pre>$ brew install Octo CLI</pre></code>
            </div>
            <p className='text-sm text-[#ac87eb]'><pre>More install options</pre></p>
        </div>
    </div>
  )
}

export default Heropage
