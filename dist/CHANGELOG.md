## Changelog
* 8e57615e85bba506a9acc3af927110072336b396 Add README
* 8353c4174fcb1062606ddb9f97c8ce4fc4fb0b11 Add ViewSnapshot and helper functions to view_clean.go
* 7d597873f94392f85c6c1c66158d8c7040938834 Add file preview features to README
* 5732ef738c4228d9424c83ba8ca34af3e1607b65 Add homebrew install instructions
* 2f3b3502241347b43634320cdeed4c1c94bc9cf8 Add safety check: only render preview if Valid flag is set
* f928c5c31a01d5ccde0ca52c35c185a3385a6413 Add syntax highlighting, nested repo support, and UI polish
* 7c2d0fcefae45fbb5208e720cbdea05b866ee6ec Add vision.md to perch
* 299a76be7a055211fea9df5551e6c04f7ef89cb9 Correct diff display to inline
* b35b675a02e4139dc0a7a80a997665ff7a679687 Expand directories into individual files
* bf9aacad8588923026935be7d58327a21f5e1809 Fix POC display issues
* 2c88d940f45b85a29cf67e6c0edba0cc6329ccd4 Fix infinite recursion in gitCmd helper
* 16ecc14f56712f199b0f56165f1a77e85ffe8c51 Fix scroll bounds checking and add caching to prevent slow file re-loads
* 908dc79ba35c507aa88921e09f44f9f67a9c3d39 Fix: only show files (not dirs), responsive width
* d5ebad9fe6bda1b1f513d3c663904b39041037e4 Fix: use -uall to expand untracked directories
* d7e46e608cecac0a0a484db5b7300d0a9570b6bc Implement Go TUI with bubbletea
* 1ed5de43265f5edf496a82140a9c5f9e39833b96 Improve homebrew formula test
* 91e386f41824afea63c67fd42d218371b92db73d Initial scaffold with bash POC
* 200f9fcceec88258fd751c666ccf40a4239c9933 Mark formula as binary-only with no dependencies
* aa5562ce33dbdd2a74acbf622198d7996f891999 POC refinements: calm UI, scrolling, global access
* ffc2aeb18ed861705683e6435209ff2935080069 Rebuild: Fix duplicate header bug with clean two-pane architecture
* 4b56f536ebaf4ea84ca8d4149faec50987c4032c Refactor to use bubbletea viewport, add loading screen, UI polish
* 310d470717e90381ac496c8e3f9d74ff8088a1bc Remove stty -echo (no lock icon), add empty state message
* 592ca925c49fcda1d58f8f7c475e348300a61871 Remove test block that triggers build
* 311e23a2535a8781e3793d271d700cf815eccc62 Rename to Perch
* cd5c42d3f7fa5d3e2ae19d481342afde4539d806 Revise README for clarity and conciseness
* d5a69efce2a74dfedb54c163bf1da4d887b454b4 Show directories and submodules, preview shows contents
* c27909280015ce525f1507c487a8942a99fef6e0 Simplify preview rendering to use raw lines, add bounds checking
* fadcc85ab4bffe50b729ca33f39f311061607d90 Simplify viewport, add bounds checks, prevent duplicate renders
* baff91df677cb9e4b5f5c22293e1dd31b45dfba5 UI polish: Catppuccin syntax highlighting, scroll preservation, auto-select newest
* 89fea53211364bf38e7ac33e6fb7a839a5d85da2 Update README with personal context
* 7e55ba6b62f2cf9d5e018bb26863dff5144b370b Use --no-optional-locks to avoid git lock contention
