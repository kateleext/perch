class Perch < Formula
  desc "Minimal file viewer for coding agents"
  homepage "https://github.com/kateleext/perch"
  version "0.0.6"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/kateleext/perch/releases/download/v0.0.6/perch_darwin_arm64.tar.gz"
      sha256 "3c1e9628d72c2a456f10cb8ab319006ced20ef40231244f7caf02ce259765032"
    end
    if Hardware::CPU.intel?
      url "https://github.com/kateleext/perch/releases/download/v0.0.6/perch_darwin_amd64.tar.gz"
      sha256 "d6f4076f367480157302545c4284f4ccf3ba5333f50ac8bd807474ef448ee2ad"
    end
  end

  on_linux do
    if Hardware::CPU.arm? && Hardware::CPU.is_64_bit?
      url "https://github.com/kateleext/perch/releases/download/v0.0.6/perch_linux_arm64.tar.gz"
      sha256 "293e81509878f7f381dd5af1bc2b55939db297378968072f060f8461434a15cb"
    end
    if Hardware::CPU.intel? && Hardware::CPU.is_64_bit?
      url "https://github.com/kateleext/perch/releases/download/v0.0.6/perch_linux_amd64.tar.gz"
      sha256 "1fd8ba52332b3c6ef95252211cacdc48d0c174ac8c73bd468a12362e20b671d8"
    end
  end

  def install
    bin.install "perch"
  end
end
