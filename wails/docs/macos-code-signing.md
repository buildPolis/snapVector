# 讓 macOS 只問你一次：Wails App 的 Developer ID 簽章實戰

> 為什麼你的 `wails dev` 每次重跑都要你重新授權 Screen Recording？答案在 cdhash，解法在 Apple Developer Program。

---

## 起因：一個讓人抓狂的小細節

如果你用 Wails 在 macOS 上做桌面 app，某天一定會遇到這個場景：

> 改一行 Go 程式 → `wails dev` → 點截圖按鈕 → **macOS 又跳出 Screen Recording 授權框** → 授權 → 重跑一次截圖 → 沒事 → 再改一行 → **又跳了**。

你會開始懷疑是自己程式寫爛了、是 Wails 的 bug、是 macOS 壞掉。都不是。

真正的元凶是 **TCC（Transparency, Consent, and Control）**——macOS 專門管隱私權限的資料庫——以及你還沒幫 app 做 **code signing**。

這篇文章會帶你：

1. 弄清楚為什麼會這樣（30 秒）
2. 用你的 Apple Developer 帳號把簽章跑起來（15 分鐘）
3. 區分 Developer ID 跟 Mac App Store 的差別（不要選錯路）

如果你正好有 Apple Developer 帳號、正好在做 Wails / Electron / Tauri / 任何 Go + WebView 的桌面 app，這篇就是為你寫的。

---

## Part 1：為什麼每次都要重新授權

macOS 用 **cdhash**（code directory hash，約等於 executable 的內容指紋）來識別 app。TCC 把你的授權記錄綁在這個 cdhash 上。

規則很簡單：

> **cdhash 變了 → TCC 視為新的 app → 重新問一次。**

下表是我實測的結果：

| Build 方式 | cdhash 穩定性 | 重新授權頻率 |
|---|---|---|
| `wails dev`（熱重編） | 每次 rebuild 都變 | **幾乎每次** |
| `wails build`（無簽章） | timestamp、buildid 影響 | 常常 |
| `wails build` + **ad-hoc sign** | 還是每次變 | 常常 |
| `wails build` + **Developer ID** | 綁 Team ID + Bundle ID | **一次永久** ✅ |

注意 Bundle ID 相同**不會**讓 TCC 跳過確認——TCC 第一層比對是 cdhash，Bundle ID 只是讓 macOS 把舊 entry 搬過來問你「要不要沿用」。

**結論**：只要你沒簽章、或只用 ad-hoc sign，就註定要一直跳。唯一根治的方法是 Developer ID Application 憑證。

---

## Part 2：取得 Developer ID Application 憑證

### Step 1：在 Xcode 產憑證（最省事）

打開 Xcode → **Settings** → **Accounts** → 登入你的 Apple ID。

選你的帳號 → 右下角 **Manage Certificates**。

左下角 `+` → 選 **Developer ID Application**。

Xcode 會自動幫你產 key pair、簽發憑證、裝進 Keychain。全程不用離開 Xcode。

> **如果你沒有 Xcode**：用 Keychain Access → `Certificate Assistant → Request a Certificate From a Certificate Authority` 產 CSR，再去 [Apple Developer Certificates 頁面](https://developer.apple.com/account/resources/certificates) 手動上傳。多幾個點擊而已。

### Step 2：確認憑證進 Keychain 了

```bash
security find-identity -v -p codesigning
```

你應該看到類似這行：

```
1) ABC123DEF456789...  "Developer ID Application: Your Name (TEAMID)"
```

把引號內整串複製起來——這是你下一步 `codesign` 指令要用的 **identity string**。

---

## Part 3：寫好 entitlements

Wails 內部用 WebKit，而 WebKit 需要 JIT。預設的 hardened runtime 會把 JIT 擋掉，app 一啟動就 crash。

在 `wails/build/darwin/entitlements.plist` 建立這個檔案：

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN"
  "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>com.apple.security.cs.allow-jit</key>
    <true/>
    <key>com.apple.security.cs.allow-unsigned-executable-memory</key>
    <true/>
    <key>com.apple.security.cs.disable-library-validation</key>
    <true/>
</dict>
</plist>
```

每個 key 的意義：

- `allow-jit`：WebKit 的 JavaScript JIT。
- `allow-unsigned-executable-memory`：Go runtime 在某些情況會分配可執行記憶體。
- `disable-library-validation`：讓 app 能載入非 Apple 簽章的 dylib（很多 Go binding 需要）。

> **注意**：這三個 entitlement 都是「弱化 hardened runtime」的 opt-out。對個人工具 app 沒問題；如果你的 app 涉及敏感資料，考慮能不能移掉 `disable-library-validation`。

---

## Part 4：簽章 + 驗證

寫成一個 shell script 放在 `wails/scripts/sign-macos.sh`：

```bash
#!/usr/bin/env bash
set -euo pipefail

IDENTITY="Developer ID Application: Your Name (TEAMID)"
APP="build/bin/snapvector.app"
ENTITLEMENTS="build/darwin/entitlements.plist"

# 1. Reproducible-ish build
wails build -platform darwin/universal -clean \
  -ldflags "-buildid= -s -w" -trimpath

# 2. Sign
codesign --sign "$IDENTITY" \
  --force --deep --timestamp --options runtime \
  --entitlements "$ENTITLEMENTS" \
  "$APP"

# 3. Verify
codesign --verify --deep --strict --verbose=2 "$APP"
spctl --assess --type execute --verbose "$APP"

echo "✅ Signed: $APP"
```

`chmod +x wails/scripts/sign-macos.sh` 之後每次發佈就跑這支。

旗標重點：

| 旗標 | 為什麼 |
|---|---|
| `--force` | 覆寫舊簽章（重 build 時必要） |
| `--deep` | 遞迴簽章 `.app` bundle 內所有 helper、framework |
| `--timestamp` | 向 Apple 的 timestamp server 拿時戳（notarization 必要） |
| `--options runtime` | 啟用 hardened runtime（notarization 必要） |
| `--entitlements` | 套用剛剛寫的 entitlements.plist |

### 驗證通過長什麼樣

`codesign --verify` 成功只會印幾行 `valid on disk`、`satisfies its Designated Requirement`。

`spctl --assess` 如果你**還沒 notarize**，會看到：

```
rejected (the code is valid but does not seem to be an app that is developed by a
          known developer for distribution to others ...)
```

自己機器測試**不用管這個**——TCC 認 Developer ID 簽章就夠了，Gatekeeper 拒絕只影響「從網路下載給別人用」的情境。

---

## Part 5：簽完之後發生什麼事

打開剛簽好的 app，點截圖 → **跳一次 Screen Recording 授權** → 點授權。

**接下來你重新 build、換 Go code、甚至改 JS 前端——只要 Developer ID identity 沒變，授權都會沿用。**

這就是簽章的魔力：TCC 不再看 cdhash 本身，而是看「這個 cdhash 是否由某個信任的 identity 簽出」。Identity 穩定，授權就穩定。

---

## Part 6：這樣可以上架 Mac App Store 嗎？

**不行。** Developer ID 跟 Mac App Store 是**兩條完全不同的路**。

| 項目 | Developer ID | Mac App Store |
|---|---|---|
| 憑證 | Developer ID Application | 3rd Party Mac Developer Application |
| 散佈 | 你自己的網站、GitHub、DMG | 只能透過 App Store |
| 審核 | notarization（機器掃） | App Review（人工） |
| Sandbox | **不強制** | **強制** |
| 抽成 | 0% | 15–30% |
| 自動更新 | 你自己做（Sparkle） | App Store 內建 |

**為什麼很多 Wails/Electron 工具選 Developer ID**：

1. App Sandbox 強制之下，很多 CLI 整合、全域 hotkey、subprocess 會壞掉。
2. 不用等 App Review。
3. 不用被抽 15%。
4. 使用者下載 DMG、拖到 Applications——這就是 Rectangle、Raycast、Alfred、Maccy 的分發方式。

**什麼時候該上 App Store**：

- 你的 app 主要使用者是非技術背景的一般消費者。
- 你想吃到 App Store 的 discovery 流量。
- 你願意把 capture/hotkey/CLI 那些東西砍掉或大改。

對絕大多數開發者工具、生產力 app，**Developer ID 才是對的答案**。

---

## Part 7（選配）：Notarization——要給別人用才做

自己機器測試不用 notarize。但只要 app 會從網路下載給別人安裝，Gatekeeper 會擋——除非你 notarize 過。

### Step 1：產 App-Specific Password

[appleid.apple.com](https://appleid.apple.com) → Sign-in and Security → App-Specific Passwords → `+` → 命名 `notarytool`。

### Step 2：一次性存進 keychain profile

```bash
xcrun notarytool store-credentials "AC_SNAPVECTOR" \
  --apple-id you@example.com \
  --team-id TEAMID \
  --password <app-specific-password>
```

### Step 3：每次發佈

```bash
# 打包成 zip（notarytool 不吃 .app 目錄，只吃 zip/dmg/pkg）
ditto -c -k --keepParent build/bin/snapvector.app snapvector.zip

# 提交（會等到結果出來再返回）
xcrun notarytool submit snapvector.zip \
  --keychain-profile "AC_SNAPVECTOR" \
  --wait

# 成功後把 notarization ticket 釘回 app
xcrun stapler staple build/bin/snapvector.app
```

Stapler 成功後，使用者下載你的 DMG、第一次打開——**Gatekeeper 不會跳警告**，TCC 也只會問一次權限。這才是使用者預期的體驗。

---

## TL;DR

1. `wails dev` 每次跳授權是 TCC 看 cdhash 在判斷，不是 bug。
2. 只有 **Developer ID Application** 憑證能讓 cdhash 背後的 identity 穩定。
3. 三個步驟：Xcode 產憑證 → 寫 entitlements.plist → `codesign --force --deep --timestamp --options runtime`。
4. 自己用不用 notarize；要給別人下載才需要。
5. **Developer ID ≠ Mac App Store**。你要的是前者。

---

## 延伸閱讀

- [Apple — Developer ID and Gatekeeper](https://developer.apple.com/developer-id/)
- [Apple — Notarizing macOS software before distribution](https://developer.apple.com/documentation/security/notarizing_macos_software_before_distribution)
- [Wails v2 — macOS Distribution](https://wails.io/docs/guides/mac-app-store/) ← 注意這篇官方文件是 App Store 版，Developer ID 流程要自己拼
- `man codesign` / `man notarytool`——文件很密、但該有的都有

祝你再也不用每 10 分鐘點一次「允許」。
