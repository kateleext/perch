class Perch < Formula
  desc "Minimal file viewer for coding agents"
  homepage "https://github.com/kateleext/perch"
  version "0.0.7"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/kateleext/perch/releases/download/v0.0.7/perch_darwin_arm64.tar.gz"
      sha256 "b8ef93f083250803081e5e68b40b49c8489d89d8f176c320c7bea850641887e8"
    end
    if Hardware::CPU.intel?
      url "https://github.com/kateleext/perch/releases/download/v0.0.7/perch_darwin_amd64.tar.gz"
      sha256 "ff2d417c7301f2a8cd3eba4e4e3945040de0813159cbfcfb0982194742a14a66"
    end
  end

  on_linux do
    if Hardware::CPU.arm? && Hardware::CPU.is_64_bit?
      url "https://github.com/kateleext/perch/releases/download/v0.0.7/perch_linux_arm64.tar.gz"
      sha256 "a57ad8ae46873c5d0c3dc89e5bef75c9786c9262eba59965d07a9b0a7bee3977"
    end
    if Hardware::CPU.intel? && Hardware::CPU.is_64_bit?
      url "https://github.com/kateleext/perch/releases/download/v0.0.7/perch_linux_amd64.tar.gz"
      sha256 "ca37299f27b311808ffceffccc08c15eb084d622217e4e129880f5a2f23a36a2"
    end
  end

  def install
    bin.install "perch"
  end
end
