cask "danmo-work" do
  version "0.9.49"
  sha256 "234907ddbc9fb27f4470af154b98b600b35292049b1fdaba93dffd3f79c1bea2"

  url "https://github.com/danmo-ai/danmo-work/releases/download/v#{version}/Danmo.Work_#{version}_arm64.dmg"
  name "Danmo Work"
  desc "Agent workbench for shipping real work products"
  homepage "https://github.com/danmo-ai/danmo-work"

  livecheck do
    url "https://github.com/danmo-ai/danmo-work/releases"
    strategy :github_latest
  end

  auto_updates true
  depends_on macos: ">= :ventura"
  depends_on arch: :arm64

  app "Danmo Work.app"

  zap trash: [
    "~/.danmo-work",
    "~/Library/Application Support/com.danmo.work",
    "~/Library/Caches/com.danmo.work",
    "~/Library/Logs/com.danmo.work",
    "~/Library/Preferences/com.danmo.work.plist",
    "~/Library/Saved Application State/com.danmo.work.savedState",
  ]

  caveats <<~EOS
    Danmo Work is not Apple-notarized yet. On first launch, right-click
    the app in Finder and choose Open (or allow it under System Settings →
    Privacy & Security).
  EOS
end
