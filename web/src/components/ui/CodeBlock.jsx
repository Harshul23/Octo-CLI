import React from 'react';
import { TbCopy } from "react-icons/tb";
import { useCopy } from "../../hooks/useCopy.js";
import { Prism as SyntaxHighlighter } from 'react-syntax-highlighter';
// "vscDarkPlus" is the classic VS Code dark theme, perfect for your aesthetic
import { vscDarkPlus } from 'react-syntax-highlighter/dist/esm/styles/prism';

const CodeBlock = ({ code, language = 'bash' }) => {

    const { copy } = useCopy();
        const text = code;
      
        const [isClicked, setIsClicked] = React.useState(false);
      
        const handleCopyClick = () => {
          copy(text);
          setIsClicked(true);
          setTimeout(() => setIsClicked(false), 150);
        };

  return (
    <div className="rounded-lg w-full flex items-center justify-between overflow-hidden border border-neutral-800 my-4 shadow-lg">      
      <SyntaxHighlighter
        language={language}
        style={vscDarkPlus}
        customStyle={{
          margin: 0,
          padding: '1.5rem',
          fontSize: '0.9rem',
          lineHeight: '1.5',
          backgroundColor: '#0d0d0d', // Matches your deep black theme
        }}
      >
        {code}
      </SyntaxHighlighter>
      <TbCopy
        onClick={handleCopyClick}
        className='mx-6'
        size={22}
        style={{
        transition: 'transform 0.15s',
        transform: isClicked ? 'scale(0.85)' : 'scale(1)'
    }}
    />
    </div>
  );
};

export default CodeBlock;