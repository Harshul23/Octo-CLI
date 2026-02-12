import ScrollZoomVideo from "../video/videoframe.jsx";
import video from "../../assets/octo-final.mp4"

const Content = () => {

  return (
    <div className="w-full h-[150%] flex justify-center items-start mt-10 bg-black">
      <ScrollZoomVideo src={video} />
    </div>
  );
};

export default Content;
