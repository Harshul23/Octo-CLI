import React from "react";
import { FaGithub } from "react-icons/fa";
import { useState } from "react";

const navItems = [
  { label: "Docs"},
  { label: "Changelog"}
];

const Navbar = () => {

  const [theme, setTheme] = useState("dark");
  const toggleTheme = () => {
    setTheme(prev => (prev === "light" ? "dark" : "light"));
  };


  return (
    <div className={`flex items-center z-20 justify-between w-full h-[10vh] px-2 ${theme == 'dark' ? "bg-black" : "bg-white"}`}>
      <div className="flex items-center w-[21em] h-full justify-between">
        <div className="flex gap-2">
          <img
            src={theme == 'dark' ? '/dark-octo.svg' : '/light-octo.svg'}
            alt="Octo CLI"
            className="inline-block w-10 h-10 ml-3"
          />
          <span className={theme == 'dark' ? "text-white text-4xl font-bold" : "text-black text-4xl font-bold"}>Octo</span>
        </div>
        <div className="flex items-center mt-3 gap-6 h-full">

          {navItems.map((item, index) => {
            return (
              <div key={index} className="flex items-center  h-full">
                <button
                  onClick={() => alert("Hello")}
                  className={`flex justify-center items-center h-full z-10 text-xl font-normal ${theme == 'dark' ? "text-[#dadada]" : "text-[#292929]"}`}
                >{item.label}
                </button>
              </div>
            );
          })}
        </div>
      </div>

      <div className="flex items-center h-full gap-4 px-3 mt-3">
        <div className="text-white flex gap-2">
          <FaGithub size={20} className={theme == 'dark' ? "text-white" : "text-black"}/>
          <span className={`text-sm font-normal ${theme == 'dark' ? "text-white" : "text-black"}`}>100 K</span>
        </div>
        <div className={`h-4 rounded-sm border ${theme == 'dark' ? "border-[#ffffff9c]" : "border-[#00000099]"}`}></div>
        <button onClick={toggleTheme} className="h-5 w-5">
          <svg className={theme === 'dark' ? "text-[white]" : "text-[black] stroke-black" } xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="white" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path stroke="none" d="M0 0h24v24H0z" fill="none"></path><path d="M12 12m-9 0a9 9 0 1 0 18 0a9 9 0 1 0 -18 0"></path><path d="M12 3l0 18"></path><path d="M12 9l4.65 -4.65"></path><path d="M12 14.3l7.37 -7.37"></path><path d="M12 19.6l8.85 -8.85"></path></svg>
        </button>
      </div>
    </div>
  );
};

export default Navbar;
