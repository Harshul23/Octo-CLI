import React from "react";
import { FaGithub } from "react-icons/fa";

const navItems = [
  { label: "Docs"},
  { label: "Changelog"}
];

const Navbar = () => {

  return (
    <div className="flex items-center justify-between w-full h-[10vh] px-2">
      <div className="flex items-center w-[30%] h-full justify-between">
        <div className="flex gap-2">
          <img
            src="/octo-cli.svg"
            alt="Octo CLI"
            className="inline-block w-10 h-10 ml-3"
          />
          <span className="font-bold text-white text-4xl">Octo</span>
        </div>
        <div className="flex items-center mt-3 gap-6 h-full">
          {navItems.map((item, index) => {
            return (
              <div key={index} className="flex items-center  h-full">
                <button
                  onClick={() => alert("Hello")}
                  className={`flex justify-center items-center h-full text-[#ffffffae] text-xl font-normal hover:shadow-lg hover:text-white`}
                >{item.label}
                </button>
              </div>
            );
          })}
        </div>
      </div>

      <div className="flex items-center h-full gap-4 px-3 mt-3">
        <div className="text-white flex gap-2">
          <FaGithub size={20}/>
          <span className="text-sm font-normal">100 K</span>
        </div>
        <div className="h-4 rounded-sm border border-[#ffffffa3]"></div>
        <svg className="text-[#ffffff]" xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="white" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="size-4.5"><path stroke="none" d="M0 0h24v24H0z" fill="none"></path><path d="M12 12m-9 0a9 9 0 1 0 18 0a9 9 0 1 0 -18 0"></path><path d="M12 3l0 18"></path><path d="M12 9l4.65 -4.65"></path><path d="M12 14.3l7.37 -7.37"></path><path d="M12 19.6l8.85 -8.85"></path></svg>
      </div>
    </div>
  );
};

export default Navbar;
