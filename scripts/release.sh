#!/usr/bin/env bash
set -euo pipefail

# ============================================================
# release.sh — 一键构建多平台二进制并发布到 GitHub Release
#
# 用法:
#   ./scripts/release.sh              # 使用最新 git tag 作为版本
#   ./scripts/release.sh v1.3.0       # 指定版本号（会自动创建 tag）
#
# 前置条件:
#   - 已安装 go、gh (GitHub CLI) 并已登录
#   - GitHub remote 名称为 vikingleo（可通过环境变量 GH_REMOTE 覆盖）
# ============================================================

APP_NAME="agent-team-monitor"
BUILD_DIR="bin"
ENTRY="./cmd/monitor"
GH_REMOTE="${GH_REMOTE:-vikingleo}"
GH_REPO="${GH_REPO:-vikingleo/agent-team-monitor}"

# 目标平台
PLATFORMS=(
  "darwin/amd64"
  "darwin/arm64"
  "linux/amd64"
  "linux/arm64"
  "windows/amd64"
  "windows/arm64"
)

# ---- 版本号 ----
if [[ $# -ge 1 ]]; then
  VERSION="$1"
else
  # 从 git tag 获取最新版本
  VERSION=$(git describe --tags --abbrev=0 2>/dev/null || echo "")
  if [[ -z "$VERSION" ]]; then
    echo "错误: 未找到 git tag，请指定版本号，例如: ./scripts/release.sh v1.3.0"
    exit 1
  fi
fi

echo "=========================================="
echo "  发布版本: ${VERSION}"
echo "  远程仓库: ${GH_REMOTE} (${GH_REPO})"
echo "=========================================="

# ---- 确认 ----
read -rp "确认发布? (y/N) " confirm
if [[ "$confirm" != "y" && "$confirm" != "Y" ]]; then
  echo "已取消"
  exit 0
fi

# ---- 创建 tag（如果不存在）----
if ! git rev-parse "$VERSION" >/dev/null 2>&1; then
  echo ">> 创建 tag: ${VERSION}"
  git tag "$VERSION"
fi

# ---- 推送 tag ----
echo ">> 推送 tag 到 ${GH_REMOTE}..."
git push "$GH_REMOTE" "$VERSION"

# ---- 更新 main.go 中的版本号 ----
SEMVER="${VERSION#v}"
if grep -q "appVersion" cmd/monitor/main.go; then
  sed -i.bak "s/appVersion = \".*\"/appVersion = \"${SEMVER}\"/" cmd/monitor/main.go
  rm -f cmd/monitor/main.go.bak
fi

# ---- 清理 & 构建 ----
echo ">> 清理旧构建..."
rm -rf "${BUILD_DIR}"
mkdir -p "${BUILD_DIR}"

echo ">> 开始多平台构建..."
ARTIFACTS=()
for platform in "${PLATFORMS[@]}"; do
  IFS='/' read -r goos goarch <<< "$platform"
  output="${BUILD_DIR}/${APP_NAME}-${goos}-${goarch}"
  if [[ "$goos" == "windows" ]]; then
    output="${output}.exe"
  fi

  echo "   构建 ${goos}/${goarch}..."
  GOOS="$goos" GOARCH="$goarch" go build -ldflags="-s -w" -o "$output" "$ENTRY"
  ARTIFACTS+=("$output")
done

echo ">> 构建完成:"
ls -lh "${BUILD_DIR}/"

# ---- 生成 Release Notes ----
NOTES="## ${VERSION}

### Downloads

| Platform | Architecture | File |
|----------|-------------|------|
| macOS | Intel (amd64) | \`${APP_NAME}-darwin-amd64\` |
| macOS | Apple Silicon (arm64) | \`${APP_NAME}-darwin-arm64\` |
| Linux | amd64 | \`${APP_NAME}-linux-amd64\` |
| Linux | arm64 | \`${APP_NAME}-linux-arm64\` |
| Windows | amd64 | \`${APP_NAME}-windows-amd64.exe\` |
| Windows | arm64 | \`${APP_NAME}-windows-arm64.exe\` |

### Commits since last release
$(git log "$(git describe --tags --abbrev=0 "${VERSION}^" 2>/dev/null || git rev-list --max-parents=0 HEAD)"..HEAD --oneline 2>/dev/null || echo "Initial release")
"

# ---- 创建 GitHub Release ----
echo ">> 创建 GitHub Release: ${VERSION}..."
gh release create "$VERSION" \
  "${ARTIFACTS[@]}" \
  --repo "$GH_REPO" \
  --title "$VERSION" \
  --notes "$NOTES"

echo ""
echo "=========================================="
echo "  发布成功! ${VERSION}"
echo "  https://github.com/${GH_REPO}/releases/tag/${VERSION}"
echo "=========================================="
