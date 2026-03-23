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
#   - 本地存在指向 ${GH_REPO} 的 GitHub remote（可通过环境变量 GH_REMOTE 覆盖）
# ============================================================

APP_NAME="agent-team-monitor"
PROJECT_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
BUILD_DIR="${PROJECT_ROOT}/bin"
ENTRY="./cmd/monitor"
GH_REMOTE="${GH_REMOTE:-}"
GH_REPO="${GH_REPO:-vikingleo/agent-team-monitor}"

PLATFORMS=(
  "darwin/amd64"
  "darwin/arm64"
  "linux/amd64"
  "linux/arm64"
  "windows/amd64"
  "windows/arm64"
)

matches_github_repo() {
  local url="$1"
  local repo="${GH_REPO}"

  [[ "${url}" =~ ^git@github\.com:${repo}(\.git)?$ ]] \
    || [[ "${url}" =~ ^https://github\.com/${repo}(\.git)?$ ]] \
    || [[ "${url}" =~ ^ssh://git@github\.com/${repo}(\.git)?$ ]]
}

resolve_git_remote() {
  if [[ -n "${GH_REMOTE}" ]]; then
    if git remote get-url "${GH_REMOTE}" >/dev/null 2>&1; then
      return
    fi
    echo "错误: 指定的 GH_REMOTE 不存在: ${GH_REMOTE}"
    echo "可用远程仓库:"
    git remote -v || true
    exit 1
  fi

  local remote
  local remote_url
  while IFS= read -r remote; do
    remote_url="$(git remote get-url "${remote}" 2>/dev/null || true)"
    if [[ -n "${remote_url}" ]] && matches_github_repo "${remote_url}"; then
      GH_REMOTE="${remote}"
      return
    fi
  done < <(git remote)

  echo "错误: 未找到指向 GitHub 仓库 ${GH_REPO} 的 git remote"
  echo "请先执行:"
  echo "  git remote add github git@github.com:${GH_REPO}.git"
  exit 1
}

ensure_release_source_version() {
  local version="$1"
  local source_file="${2}/cmd/monitor/main.go"

  if rg -q '^var[[:space:]]+appVersion[[:space:]]*=' "${source_file}"; then
    RELEASE_LDFLAGS="-s -w -X main.appVersion=${version}"
    return
  fi

  echo ">> 旧版源码使用静态版本号，正在隔离构建目录中写入 ${version}..."
  sed -i.bak "s/appVersion = \".*\"/appVersion = \"${version}\"/" "${source_file}"
  rm -f "${source_file}.bak"
  RELEASE_LDFLAGS="-s -w"
}

setup_release_worktree() {
  RELEASE_WORKTREE="$(mktemp -d "${TMPDIR:-/tmp}/${APP_NAME}-release-XXXXXX")"
  git worktree add --detach "${RELEASE_WORKTREE}" "${VERSION}" >/dev/null
}

cleanup() {
  if [[ -n "${RELEASE_WORKTREE:-}" && -d "${RELEASE_WORKTREE}" ]]; then
    git worktree remove --force "${RELEASE_WORKTREE}" >/dev/null 2>&1 || true
  fi
}

resolve_git_remote
trap cleanup EXIT

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
if [[ -t 0 ]]; then
  read -rp "确认发布? (y/N) " confirm
  if [[ "$confirm" != "y" && "$confirm" != "Y" ]]; then
    echo "已取消"
    exit 0
  fi
else
  echo ">> 非交互环境，跳过发布确认"
fi

# ---- 创建 tag（如果不存在）----
if ! git rev-parse "$VERSION" >/dev/null 2>&1; then
  echo ">> 创建 tag: ${VERSION}"
  git tag -a "$VERSION" -m "Release ${VERSION}"
fi

# ---- 推送 tag ----
echo ">> 推送 tag 到 ${GH_REMOTE}..."
git push "$GH_REMOTE" "$VERSION"

SEMVER="${VERSION#v}"

# ---- 清理 & 构建 ----
echo ">> 清理旧构建..."
rm -rf "${BUILD_DIR}"
mkdir -p "${BUILD_DIR}"

echo ">> 准备隔离构建目录..."
setup_release_worktree
ensure_release_source_version "${SEMVER}" "${RELEASE_WORKTREE}"

echo ">> 开始多平台构建..."
ARTIFACTS=()
for platform in "${PLATFORMS[@]}"; do
  IFS='/' read -r goos goarch <<< "$platform"
  output="${BUILD_DIR}/${APP_NAME}-${goos}-${goarch}"
  if [[ "$goos" == "windows" ]]; then
    output="${output}.exe"
  fi

  echo "   构建 ${goos}/${goarch}..."
  (
    cd "${RELEASE_WORKTREE}"
    GOOS="$goos" GOARCH="$goarch" go build -ldflags="${RELEASE_LDFLAGS}" -o "$output" "$ENTRY"
  )
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

# ---- 创建 / 更新 GitHub Release ----
if gh release view "$VERSION" --repo "$GH_REPO" >/dev/null 2>&1; then
  echo ">> 更新 GitHub Release: ${VERSION}..."
  gh release edit "$VERSION" \
    --repo "$GH_REPO" \
    --title "$VERSION" \
    --notes "$NOTES"
  gh release upload "$VERSION" \
    "${ARTIFACTS[@]}" \
    --repo "$GH_REPO" \
    --clobber
else
  echo ">> 创建 GitHub Release: ${VERSION}..."
  gh release create "$VERSION" \
    "${ARTIFACTS[@]}" \
    --repo "$GH_REPO" \
    --title "$VERSION" \
    --notes "$NOTES" \
    --verify-tag
fi

echo ""
echo "=========================================="
echo "  发布成功! ${VERSION}"
echo "  https://github.com/${GH_REPO}/releases/tag/${VERSION}"
echo "=========================================="
