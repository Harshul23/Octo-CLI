import { ChevronDown } from "lucide-react";
import { useState } from "react";
import { NavLink } from "react-router-dom";

const Sidebar = () => {
  const [dropdown, setDropdown] = useState(true);

  return (
    <div className={`flex flex-col fixed h-8/10 mt-20 border-r border-neutral-700 w-2/10 px-4 py-4 bg-black`}>
      <div className="flex justify-between items-center">
        <span className="text-xl font-medium text-white">Get Started</span>

        <ChevronDown
          onClick={() => setDropdown(!dropdown)}
          className={`transition-transform ${
            dropdown ? "rotate-0" : "-rotate-90"
          }`}
          size={20}
        />
      </div>

      {dropdown && (
        <div className="border-l border-neutral-600 px-4 ml-2 mt-4">
          <NavLink
            to="/docs/installation"
            className={({ isActive }) =>
              `block text-[1.1em] mb-2 px-2 py-1 rounded-lg ${
                isActive
                  ? "text-white bg-blue-500" : "text-neutral-400 hover:text-white"
              }`
            }
          >
            Overview
          </NavLink>

          <NavLink
            to="/docs/quick-guide"
            className={({ isActive }) =>
              `block text-[1.1em] mb-2 ${
                isActive
                  ? "text-white bg-neutral-600" : "text-neutral-400 hover:text-white"
              }`
            }
          >
            Quickstart
          </NavLink>
        </div>
      )}
    </div>
  );
};

export default Sidebar