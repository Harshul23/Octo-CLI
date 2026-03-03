import Onpage from "./onpage.jsx"

const Overview = () => {
  return (
    <div className='bg-black flex'>
      <div className="h-full mt-20 ml-60 w-5/10 text-white pt-2 bg-black flex flex-col gap-10">
        <div className='w-full'>
          <span className='text-[2.8em] font-medium'>Octo  CLI  documentation</span>
            <p className='text-[1.1em] font-light py-4 text-neutral-300'>Octo CLI is an open-source, terminal-first tool that automates local execution of any project. It analyzes your codebase, detects the tech stack, and prepares the right runtime whether Docker, Nix, or shell without manual configuration. 
            <br /> <br /> Built for real-world repositories, Octo understands your project structure, generates the required setup, and runs your app the way it was meant to be run. 
            <br /> <br /> Now local execution is automated from detection to deployment.</p>
        </div>
        <div className="flex flex-col gap-8">
          <div className="flex flex-col gap-2">
            <span className="text-4xl font-medium">Get Started</span>
            <p className="text-lg font-light text-neutral-300">Begin your journey with Octo CLI by setting up your environment and learning the basics.</p>
            <ul className="list-disc pl-4 list-inside text-neutral-300 flex flex-col gap-1 text-lg font-light">
              <li>Install</li>
              <li>Initialize</li>
              <li>Run</li>
              <li>Core Commands</li>
            </ul>
          </div>
          <div className="flex flex-col gap-2">
            <span className="text-4xl font-medium">How it works</span>
            <p className="text-lg font-light text-neutral-300">Octo automates the standard developer "inner loop" through a multi-phase orchestrator:</p>
            <ul className="list-disc pl-4 list-inside text-neutral-300 flex flex-col gap-1 text-lg font-light">
                <li>Thermal Detection</li>
                <li>Environment Provisioning</li>
                <li>Dependency Check</li>
                <li>Port Management</li>
                <li>Execution</li>
            </ul>
          </div>

          <div className="flex flex-col gap-2">
            <span className="text-4xl font-medium">Configuration</span>
            <p className="text-lg font-light text-neutral-300">The blueprint defines how Octo interacts with your software.</p>
          </div>

          <div className="flex flex-col gap-2">
            <span className="text-4xl font-medium">Thermal Management</span>
            <p className="text-lg font-light text-neutral-300">Built with a deep understanding of hardware.</p>
          </div>

          <div className="flex flex-col gap-2">
            <span className="text-4xl font-medium">FAQs</span>
            <p className="text-lg font-light text-neutral-300">Answers to those common questions and solutions to frequent problems encountered while using Octo CLI.</p>
          </div>

        </div>
      </div>
      <div>
        <Onpage />
      </div>
    </div>
  )
}

export default Overview