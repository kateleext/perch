class Perch < Formula
  desc "Minimal file viewer for coding agents"
  homepage "https://github.com/kateleext/perch"
  version "0.0.4"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/kateleext/perch/releases/download/v0.0.4/perch_darwin_arm64.tar.gz"
      sha256 "0281a807d8a8d4334a2bd9d69634a12863e19b46639ecfb39a70fbff328793ba"
    end
    if Hardware::CPU.intel?
      url "https://github.com/kateleext/perch/releases/download/v0.0.4/perch_darwin_amd64.tar.gz"
      sha256 "8f196ea5795ca95f5f273e10110be8be222089ccfcaa59060794bee50b3f9d35"
    end
  end

  on_linux do
    if Hardware::CPU.arm? && Hardware::CPU.is_64_bit?
      url "https://github.com/kateleext/perch/releases/download/v0.0.4/perch_linux_arm64.tar.gz"
      sha256 "21593a52d2bd4d8d692d7b8d06ef4403c18f515c22612e115ed9d9c5fb2c7b1b"
    end
    if Hardware::CPU.intel? && Hardware::CPU.is_64_bit?
      url "https://github.com/kateleext/perch/releases/download/v0.0.4/perch_linux_amd64.tar.gz"
      sha256 "221a8e69ca4ecdbe5a90b344907b9a706092c587da863126d3cbe9d2dc710bdd"
    end
  end

  def install
    bin.install "perch"
  end
end
