class Perch < Formula
  desc "Minimal file viewer for coding agents"
  homepage "https://github.com/kateleext/perch"
  version "0.0.8"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/kateleext/perch/releases/download/v0.0.8/perch_darwin_arm64.tar.gz"
      sha256 "d399519f283ffbc9aa6881af1efc1251584d3e8ef10f3854795ff0126fce6fad"
    end
    if Hardware::CPU.intel?
      url "https://github.com/kateleext/perch/releases/download/v0.0.8/perch_darwin_amd64.tar.gz"
      sha256 "103fa2095c3ae02c50d7875812b0e944eb2fd07a1f15e7ad90ea193c87513de2"
    end
  end

  on_linux do
    if Hardware::CPU.arm? && Hardware::CPU.is_64_bit?
      url "https://github.com/kateleext/perch/releases/download/v0.0.8/perch_linux_arm64.tar.gz"
      sha256 "e18265f269a9c45467ee08ccf31ee851732c0973f2a0eaf3aac5e6caa8e3c51a"
    end
    if Hardware::CPU.intel? && Hardware::CPU.is_64_bit?
      url "https://github.com/kateleext/perch/releases/download/v0.0.8/perch_linux_amd64.tar.gz"
      sha256 "84f521d5cb87b36a12bfcbf8f89fce90e747328e11a6e3b3914e2bc82f9c0fb9"
    end
  end

  def install
    bin.install "perch"
  end
end
