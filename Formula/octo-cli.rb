class OctoCli < Formula
  desc "Execution intelligence engine for software repositories and AI agents"
  homepage "https://github.com/Octo-sh/Octo-CLI"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/Octo-sh/Octo-CLI/releases/download/v1.2.2/octo_v1.2.2_darwin_arm64.tar.gz"
      sha256 "ba4781f178208c5875911a30078b6182b4edc52497358797009eb1a301dd81ed"
    else
      url "https://github.com/Octo-sh/Octo-CLI/releases/download/v1.2.2/octo_v1.2.2_darwin_amd64.tar.gz"
      sha256 "6a659aff39455ab7e3a58840d357c8c6f3821212cde1fbccb4c6f33c40bf7fce"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/Octo-sh/Octo-CLI/releases/download/v1.2.2/octo_v1.2.2_linux_arm64.tar.gz"
      sha256 "ff6e32444894924f8623be8e2ad1702d39801df4692e1cfb5a1f45584c3c93b2"
    else
      url "https://github.com/Octo-sh/Octo-CLI/releases/download/v1.2.2/octo_v1.2.2_linux_amd64.tar.gz"
      sha256 "75df8beed1ef4d9bc97b4671a61aa5e0fd395535e15bcb79543973842a3f79f7"
    end
  end

  def install
    bin.install "octo"
  end

  test do
    system bin/"octo", "--version"
  end
end
