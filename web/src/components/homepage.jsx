import Navbar from './navbar'
import Heropage from './heropage'
import Footer from './footer'

const Homepage = () => {
  return (
    <div className='flex flex-col w-full bg-black'>
      <Navbar />
      <Heropage />
      <Footer />
    </div>
  )
}

export default Homepage
