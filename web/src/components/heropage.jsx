import FadeText from './FadeText'
import Content from './mainContent/content.jsx'

const Heropage = () => {
  return (
    <div className='min-h-8/10 mt-44 overflow-scroll w-full flex flex-col items-center justify-center'>
        <div className='flex-col gap-6 flex items-center justify-center w-full h-full'>
            <p className='text-6xl font-medium px-[4em] text-white text-center h-full w-full'><FadeText text="Now local execution is automated from detection to deployment" /></p>
            <p className='text-2xl font-normal px-[9em] text-[#ffffffb5] text-center h-full w-full'>A single command that understands your project, prepares everything it needs, and runs it the way it was meant to.</p>
            <div className='text-sm bg-[#17131d] text-white px-4 py-2 rounded-xl border-2 border-[#ac87eb]'>
                <code><pre>$ brew install Octo CLI</pre></code>
            </div>
            <p className='text-sm text-[#ac87eb]'><pre>More install options</pre></p>
        </div>
        <Content />
    </div>
  )
}

export default Heropage
