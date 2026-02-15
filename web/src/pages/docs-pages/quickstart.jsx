import Onpage from "./onpage.jsx"
import CodeBlock from '../../components/ui/CodeBlock.jsx';

const Quickstart = () => {
  return (
   <div className='bg-black flex'>
      <div className="h-full mt-20 ml-60 w-5/10 text-white pt-2 bg-black flex flex-col gap-10">
        <div className='w-full flex flex-col gap-1'>
          <span className='text-[2.8em] font-medium'>Get Started with Octo CLI</span>
            <p className='text-[1.1em] font-light text-neutral-300'>Welcome to Octo CLI ! This guide will help you install, configure, and start using the Octo CLI to automate your local development workflow right from your terminal.</p>
        </div>
        <div className="flex flex-col gap-8">
          <div className="flex flex-col gap-2">
            <span className="text-3xl py-4">Installation</span>
            <p className="text-[1em] font-light text-neutral-300">The standard method to install and run Octo CLI uses <span className="bg-neutral-800 px-2 text-sm py-0.5 rounded-sm">brew-tap</span></p>
              <CodeBlock 
                code="brew install harshul23/tap/octo-cli" 
                language="bash" 
              />
              <p className="text-[1em] font-light text-neutral-300">Alternatively, you can also install Octo CLI using Go:</p>
              <CodeBlock 
                code="go install github.com/harshul/octo-cli/cmd@latest" 
                language="bash" 
              />
          </div>
          <div className="flex flex-col gap-2">
            <span className="text-3xl py-4">Quick Start</span>
            <p className="text-lg font-light text-neutral-300">1. Navigate to your project root.</p>
            <CodeBlock 
                code="cd your-project" 
                language="bash" 
            />

            <p className="text-lg font-light mt-6 text-neutral-300">2. Let Octo analyze your codebase, detects your tech stack, and handles the heavy lifting of environment provisioning, dependency management, and local orchestration.</p>
            <CodeBlock 
                code="octo init" 
                language="bash" 
            />

            <p  className="text-lg font-light text-neutral-300">This generates a <span className="bg-neutral-800 px-2 text-sm py-0.5 rounded-sm">.octo.yaml</span> blueprint optimized for your project's detected language and framework.</p>

             <p className="text-lg font-light mt-6 text-neutral-300">3. Run your application with just a single command</p>
            <CodeBlock 
                code="octo run" 
                language="bash" 
            />
          </div>

          <div className="flex flex-col gap-2">
            <span className="text-4xl font-medium">Commands</span>
            <span className="bg-neutral-900 px-2 py-1 w-34 rounded-xl flex items-center justify-center mt-4 text-3xl">octo init</span>
            <p className="text-lg font-light pl-2 text-neutral-300">Analyzes the codebase and generates a .octo.yaml configuration file.</p>
            <CodeBlock 
                code={`
octo init [flags]

Flags:
  -o, --output string   Output file path (default ".octo.yaml")
  -f, --force           Overwrite existing configuration
  -i, --interactive     Run in interactive mode with prompts"
                  `}
            />


            <span className="bg-neutral-900 px-2 py-1 w-34 rounded-xl flex items-center justify-center mt-4 text-3xl">octo run</span>
            <p className="text-lg font-light pl-2 text-neutral-300">Executes the software based on the .octo.yaml file.</p>
            <CodeBlock 
                code={`
octo run [flags]

Flags:
  -c, --config string   Configuration file path (default ".octo.yaml")
  -e, --env string      Environment to run (default "development")
  -b, --build           Run build step (default true)
  -w, --watch           Watch for file changes and restart
  -d, --detach          Run in detached mode (background)
                  `}
            />

          </div>

        </div>
      </div>
      <div>
        <Onpage />
      </div>
    </div>
  )
}

export default Quickstart