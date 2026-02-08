import Navbar from './navbar'
import Heropage from './heropage'

const Homepage = () => {
  return (
    <div className='flex flex-col w-full bg-black'>
      <Navbar />
      <Heropage />
    </div>
  )
}

export default Homepage
