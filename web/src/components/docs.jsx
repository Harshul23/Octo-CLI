import Sidebar from './sidebar'

const Docs = () => {
  return (
    <div className='h-screen mt-20 w-full text-white pt-2 bg-black flex'>
        <div className='pl-14 w-8/10 py-8'>
          <span className='text-[2.5em] font-medium'>Octo  CLI  documentation</span>
            <p className='text-sm text-neutral-300'>Octo CLI is an open-source, terminal-first tool that automates local execution of any project. It analyzes your codebase, detects the tech stack, and prepares the right runtime whether Docker, Nix, or shell without manual configuration. 
            <br /> <br /> Built for real-world repositories, Octo understands your project structure, generates the required setup, and runs your app the way it was meant to be run. 
            <br /> <br /> Now local execution is automated from detection to deployment.</p>
        </div>
    </div>
  )
}

export default Docs