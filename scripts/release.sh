#!/usr/bin/env bash
set -euo pipefail

# ============================================================
# release.sh — 一键构建多平台二进制并发布到 GitHub Release
#
# 用法:
#   ./scripts/release.sh              # 使用最新 git tag 作为版本
#   ./scripts/release.sh v1.3.0       # 指定版本号（会自动创建 tag）
#   ./scripts/release.sh v1.5.0 --retag-current
#                                  # 用当前 HEAD 覆盖已有版本 tag 并重发 release
#
# 前置条件:
#   - 已安装 go、gh (GitHub CLI) 并已登录
#   - 本地存在 GitHub remote，默认使用 origin（可通过 GIT_REMOTE 覆盖）
# ============================================================

APP_NAME="agent-team-monitor"
PROJECT_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
BUILD_DIR="${PROJECT_ROOT}/bin"
ENTRY="./cmd/monitor"
GIT_REMOTE="${GIT_REMOTE:-origin}"
GH_REPO="${GH_REPO:-}"
DEFAULT_GH_REPO="vikingleo/agent-team-monitor"
REMOTE_URL=""
VERSION=""
BUILD_REF=""
FORCE_RETAG_CURRENT=0
SKIP_CONFIRM=0
TAG_PUSH_FORCE=0

PLATFORMS=(
  "darwin/amd64"
  "darwin/arm64"
  "linux/amd64"
  "linux/arm64"
  "windows/amd64"
  "windows/arm64"
)

usage() {
  cat <<'EOF'
用法:
  ./scripts/release.sh [version] [options]

参数:
  version             发布版本号，例如 v1.5.0；省略时使用最新 tag

选项:
  --retag-current     将已有版本 tag 强制移动到当前 HEAD，再覆盖发布 release
  -y, --yes           跳过交互确认
  -h, --help          显示帮助

示例:
  ./scripts/release.sh
  ./scripts/release.sh v1.6.0
  ./scripts/release.sh v1.5.0 --retag-current
EOF
}

parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --retag-current)
        FORCE_RETAG_CURRENT=1
        ;;
      -y|--yes)
        SKIP_CONFIRM=1
        ;;
      -h|--help)
        usage
        exit 0
        ;;
      -*)
        echo "错误: 未知参数 $1"
        echo ""
        usage
        exit 1
        ;;
      *)
        if [[ -n "${VERSION}" ]]; then
          echo "错误: 只能指定一个版本号，收到重复参数: ${VERSION} 和 $1"
          exit 1
        fi
        VERSION="$1"
        ;;
    esac
    shift
  done
}

parse_github_repo() {
  local remote_url="$1"
  local repo_path="$remote_url"

  repo_path="${repo_path#git@github.com:}"
  repo_path="${repo_path#ssh://git@github.com/}"
  repo_path="${repo_path#https://github.com/}"
  repo_path="${repo_path#http://github.com/}"
  repo_path="${repo_path%.git}"

  if [[ "$repo_path" == */* && "$repo_path" != "$remote_url" ]]; then
    echo "$repo_path"
    return 0
  fi

  return 1
}

resolve_git_remote() {
  if ! git remote get-url "${GIT_REMOTE}" >/dev/null 2>&1; then
    local example_repo="${GH_REPO:-${DEFAULT_GH_REPO}}"
    echo "错误: 未找到名为 ${GIT_REMOTE} 的 Git remote"
    echo "请先执行:"
    echo "  git remote add ${GIT_REMOTE} git@github.com:${example_repo}.git"
    exit 1
  fi

  REMOTE_URL="$(git remote get-url "${GIT_REMOTE}")"
}

resolve_github_repo() {
  if [[ -n "${GH_REPO}" ]]; then
    return
  fi

  if GH_REPO="$(parse_github_repo "${REMOTE_URL}")"; then
    return
  fi

  echo "错误: 无法从 remote ${GIT_REMOTE} 解析 GitHub 仓库"
  echo "当前 remote URL: ${REMOTE_URL}"
  echo "请设置 GH_REPO=<owner>/<repo> 后重试"
  exit 1
}

ensure_gh_ready() {
  if ! command -v gh >/dev/null 2>&1; then
    echo "错误: 未找到 gh，请先安装 GitHub CLI"
    exit 1
  fi

  if ! gh auth status >/dev/null 2>&1; then
    echo "错误: gh 尚未登录，请先执行 gh auth login"
    exit 1
  fi
}

ensure_clean_worktree() {
  if [[ -n "$(git status --porcelain)" ]]; then
    echo "错误: 当前工作区有未提交变更，--retag-current 只能发布当前 HEAD 提交"
    echo "请先提交或清理工作区后重试"
    exit 1
  fi
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
  git worktree add --detach "${RELEASE_WORKTREE}" "${BUILD_REF}" >/dev/null
}

cleanup() {
  if [[ -n "${RELEASE_WORKTREE:-}" && -d "${RELEASE_WORKTREE}" ]]; then
    git worktree remove --force "${RELEASE_WORKTREE}" >/dev/null 2>&1 || true
  fi
}

prepare_tag() {
  local head_commit
  local tag_commit=""

  head_commit="$(git rev-parse HEAD)"

  if git rev-parse "${VERSION}" >/dev/null 2>&1; then
    tag_commit="$(git rev-parse "${VERSION}^{}")"
  fi

  if [[ "${FORCE_RETAG_CURRENT}" -eq 1 ]]; then
    ensure_clean_worktree

    if [[ -n "${tag_commit}" && "${tag_commit}" == "${head_commit}" ]]; then
      echo ">> ${VERSION} 已指向当前 HEAD，无需重写 tag"
    else
      echo ">> 将 ${VERSION} 重新指向当前 HEAD (${head_commit})..."
      git tag -fa "${VERSION}" -m "Release ${VERSION}" "${head_commit}"
      TAG_PUSH_FORCE=1
    fi
  elif [[ -z "${tag_commit}" ]]; then
    echo ">> 创建 tag: ${VERSION}"
    git tag -a "${VERSION}" -m "Release ${VERSION}" "${head_commit}"
  fi

  BUILD_REF="${VERSION}"
}

generate_release_notes() {
  local repo_dir="$1"
  local history_start
  local commits

  history_start="$(git -C "${repo_dir}" describe --tags --abbrev=0 HEAD^ 2>/dev/null || git -C "${repo_dir}" rev-list --max-parents=0 HEAD | tail -n 1)"
  commits="$(git -C "${repo_dir}" log "${history_start}"..HEAD --oneline 2>/dev/null || true)"
  if [[ -z "${commits}" ]]; then
    commits="Initial release"
  fi

  cat <<EOF
## ${VERSION}

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
${commits}
EOF
}

parse_args "$@"
resolve_git_remote
resolve_github_repo
ensure_gh_ready
trap cleanup EXIT

# ---- 版本号 ----
if [[ -z "${VERSION}" ]]; then
  # 从 git tag 获取最新版本
  VERSION=$(git describe --tags --abbrev=0 2>/dev/null || echo "")
  if [[ -z "$VERSION" ]]; then
    echo "错误: 未找到 git tag，请指定版本号，例如: ./scripts/release.sh v1.3.0"
    exit 1
  fi
fi

echo "=========================================="
echo "  发布版本: ${VERSION}"
echo "  远程仓库: ${GIT_REMOTE} (${GH_REPO})"
if [[ "${FORCE_RETAG_CURRENT}" -eq 1 ]]; then
  echo "  发布模式: 覆盖已有 tag 到当前 HEAD"
fi
echo "=========================================="

# ---- 确认 ----
if [[ "${SKIP_CONFIRM}" -eq 1 ]]; then
  echo ">> 已指定 --yes，跳过发布确认"
elif [[ -t 0 ]]; then
  read -rp "确认发布? (y/N) " confirm
  if [[ "$confirm" != "y" && "$confirm" != "Y" ]]; then
    echo "已取消"
    exit 0
  fi
else
  echo ">> 非交互环境，跳过发布确认"
fi

# ---- 准备 tag / 构建来源 ----
prepare_tag

# ---- 推送 tag ----
if [[ "${TAG_PUSH_FORCE}" -eq 1 ]]; then
  echo ">> 强制推送 tag 到 ${GIT_REMOTE}..."
  git push --force "$GIT_REMOTE" "refs/tags/${VERSION}"
else
  echo ">> 推送 tag 到 ${GIT_REMOTE}..."
  git push "$GIT_REMOTE" "$VERSION"
fi

SEMVER="${VERSION#v}"

# ---- 清理 & 构建 ----
echo ">> 清理旧构建..."
rm -rf "${BUILD_DIR}"
mkdir -p "${BUILD_DIR}"

echo ">> 准备隔离构建目录..."
setup_release_worktree
ensure_release_source_version "${SEMVER}" "${RELEASE_WORKTREE}"
echo ">> 构建来源: ${BUILD_REF} -> $(git -C "${RELEASE_WORKTREE}" rev-parse --short HEAD)"

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
NOTES="$(generate_release_notes "${RELEASE_WORKTREE}")"

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
