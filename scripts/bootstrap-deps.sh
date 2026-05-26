#!/usr/bin/env bash

set -euo pipefail

# 为独立 gobao-order 仓准备本地依赖目录，解决 gobao-pkg / gobao-proto 的 replace 依赖。
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
WORKSPACE_DIR="${ROOT_DIR}/workspace"
BRANCH="${GOBAO_REPO_BRANCH:-main}"

repos=(
  "gobao-pkg"
  "gobao-proto"
)

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "缺少命令: $1"
    exit 1
  fi
}

require_cmd git

mkdir -p "${WORKSPACE_DIR}"

for repo in "${repos[@]}"; do
  target="${WORKSPACE_DIR}/${repo}"
  remote="https://github.com/yym108/${repo}.git"

  if [ -d "${target}/.git" ]; then
    echo "更新 ${repo}"
    git -C "${target}" fetch --depth=1 origin "${BRANCH}"
    git -C "${target}" checkout -B "${BRANCH}" "origin/${BRANCH}"
  else
    echo "浅克隆 ${repo}"
    git clone --depth=1 --branch "${BRANCH}" "${remote}" "${target}"
  fi
done

echo "依赖仓已准备完成：${WORKSPACE_DIR}"
