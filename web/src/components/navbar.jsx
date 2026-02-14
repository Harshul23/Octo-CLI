import React, { useState } from "react";
import { FaGithub } from "react-icons/fa";
import { NavLink } from "react-router-dom";

const navItems = [
  { label: "Docs", path: "/docs" },
  { label: "Changelog", path: "/changelog" },
];

const Navbar = () => {
  const [theme, setTheme] = useState("dark");

  const toggleTheme = () => {
    setTheme((prev) => (prev === "light" ? "dark" : "light"));
  };

  const baseLink =
    "flex justify-center items-center h-full z-10 text-xl font-normal transition-colors duration-200";

  return (
    <div
      className={`flex fixed items-center z-20 justify-between w-full h-[10vh] px-2 ${
        theme === "dark" ? "bg-black" : "bg-white"
      }`}
    >
      <div className="flex items-center w-[21em] h-full justify-between">
        <div className="flex gap-2 items-center">
          <img
            src={theme === "dark" ? "/dark-octo.svg" : "/light-octo.svg"}
            alt="Octo CLI"
            className="inline-block w-10 h-10 ml-3"
          />
          <span
            className={`text-4xl font-bold ${
              theme === "dark" ? "text-white" : "text-black"
            }`}
          >
            Octo
          </span>
        </div>

        <div className="flex items-center mt-3 gap-6 h-full">
          {navItems.map((item, index) => (
            <div key={index} className="flex items-center h-full">
              <NavLink
                to={item.path}
                className={({ isActive }) =>
                  `${baseLink} ${
                    isActive
                      ? "text-purple-500"
                      : theme === "dark"
                      ? "text-[#c3c3c3] hover:text-white"
                      : "text-[#292929] hover:text-black"
                  }`
                }
              >
                {item.label}
              </NavLink>
            </div>
          ))}
        </div>
      </div>

      <div className="flex items-center h-full gap-4 px-3 mt-3">
        <div
          onClick={() =>
            window.open(
              "https://github.com/Harshul23/Octo-CLI",
              "_blank"
            )
          }
          className="flex gap-2 cursor-pointer"
        >
          <FaGithub
            size={20}
            className={theme === "dark" ? "text-white" : "text-black"}
          />
          <span
            className={`text-sm font-normal ${
              theme === "dark" ? "text-white" : "text-black"
            }`}
          >
            100 K
          </span>
        </div>

        <div
          className={`h-4 rounded-sm border ${
            theme === "dark"
              ? "border-[#ffffff9c]"
              : "border-[#00000099]"
          }`}
        ></div>

        <button onClick={toggleTheme} className="h-5 w-5">
          <svg
            className={
              theme === "dark"
                ? "text-white"
                : "text-black stroke-black"
            }
            xmlns="http://www.w3.org/2000/svg"
            width="20"
            height="20"
            viewBox="0 0 24 24"
            fill="none"
            stroke="white"
            strokeWidth="2"
            strokeLinecap="round"
            strokeLinejoin="round"
          >
            <circle cx="12" cy="12" r="9" />
            <path d="M12 3v18" />
            <path d="M12 9l4.65-4.65" />
            <path d="M12 14.3l7.37-7.37" />
            <path d="M12 19.6l8.85-8.85" />
          </svg>
        </button>
      </div>
    </div>
  );
};

export default Navbar;