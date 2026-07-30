class Perch < Formula
  desc "Minimal file viewer for coding agents"
  homepage "https://github.com/kateleext/perch"
  version "0.0.9"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/kateleext/perch/releases/download/v0.0.9/perch_darwin_arm64.tar.gz"
      sha256 "3b54f9fab651d1194c5a6368eecda8139ca430a8214e71f70778551c3e4577ef"
    end
    if Hardware::CPU.intel?
      url "https://github.com/kateleext/perch/releases/download/v0.0.9/perch_darwin_amd64.tar.gz"
      sha256 "f95566f7cc8e1ecbf053c73f3733b0c7a996bc9193daf6767fe065a8ced9f907"
    end
  end

  on_linux do
    if Hardware::CPU.arm? && Hardware::CPU.is_64_bit?
      url "https://github.com/kateleext/perch/releases/download/v0.0.9/perch_linux_arm64.tar.gz"
      sha256 "35caf7af827994a9b9de6b9aa05f425b09e60c3287e8c19d89480ca912fa10f8"
    end
    if Hardware::CPU.intel? && Hardware::CPU.is_64_bit?
      url "https://github.com/kateleext/perch/releases/download/v0.0.9/perch_linux_amd64.tar.gz"
      sha256 "4a351a8a4bf1e19c0988a57b86884eb44e374e03c5645e8364353d56fcd71f1c"
    end
  end

  def install
    bin.install "perch"
  end
end
