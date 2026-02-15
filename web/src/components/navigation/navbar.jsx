import React, { useEffect, useState } from "react";
import { FaGithub } from "react-icons/fa";
import { NavLink, useLocation } from "react-router-dom";

const navItems = [
  { label: "Docs", path: "/docs/overview", base: "/docs" },
  { label: "Changelog", path: "/changelog", base: "/changelog" },
];

const Navbar = () => {

  const [stars, setStars] = useState(null)
  const location = useLocation();

  useEffect(() => {
    fetch("https://api.github.com/repos/Harshul23/Octo-CLI")
      .then((res) => res.json())
      .then((data) => {
        setStars(data.stargazers_count);
      })
      .catch((err) => console.error(err));
  }, []);

  const baseLink =
    "flex justify-center items-center h-full z-10 text-xl font-normal transition-colors duration-200";

  return (
    <div className="flex fixed items-center z-20 justify-between w-full h-[10vh] py-6 px-2 bg-black">
      <div className="flex items-center w-[21em] h-full justify-between">
        <div className="flex gap-2 items-center">
          <img src="/dark-octo.svg" alt="Octo CLI" className="inline-block w-10 h-10 ml-3" />
          <NavLink to="/home" className="text-4xl font-bold text-white">
            Octo
          </NavLink>
        </div>

        <div className="flex items-center mt-1 gap-6 h-full">
          {navItems.map((item, index) => {
            // 4. Custom Active Check
            // If the current URL starts with the item's 'base', mark it active
            const isActive = location.pathname.startsWith(item.base);

            return (
              <div key={index} className="flex items-center h-full">
                <NavLink
                  to={item.path}
                  className={`${baseLink} ${
                    isActive
                      ? "text-white underline underline-offset-4"
                      : "text-neutral-400 hover:text-white"
                  }`}
                >
                  {item.label}
                </NavLink>
              </div>
            );
          })}
        </div>
      </div>

      <div className="flex items-center h-full gap-4 px-3 mr-8 mt-2">
        <div
          onClick={() => window.open("https://github.com/Harshul23/Octo-CLI", "_blank")}
          className="flex gap-2 cursor-pointer"
        >
          <FaGithub size={20} className="text-white" />
          <span className="text-sm font-normal text-white">
            {stars !== null ? stars : "..."}
          </span>
        </div>

        <div className="h-4 rounded-sm border border-[#ffffff9c]"></div>
        <div className="flex justify-center items-center font-medium text-neutral-400 hover:text-white text-lg rounded-lg">
          Feedback
        </div>
      </div>
    </div>
  );
};

export default Navbar;
