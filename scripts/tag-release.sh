#!/usr/bin/env bash
# 本地打开发布用双 tag（根模块 vX.Y.Z + 子模块 store/redis/vX.Y.Z）。
# 用法: ./scripts/tag-release.sh 0.1.0
# 依赖: 当前在目标 commit 上，且远端尚未存在同名 tag。
set -euo pipefail

V="${1:-}"
V="${V#v}"
if ! [[ "$V" =~ ^[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9.]+)?(\+[a-zA-Z0-9.]+)?$ ]]; then
	echo "usage: $0 <semver>   # e.g. 0.1.0" >&2
	exit 1
fi

git fetch origin --tags 2>/dev/null || true
if git show-ref --verify --quiet "refs/tags/v${V}"; then
	echo "tag v${V} already exists" >&2
	exit 1
fi
if git show-ref --verify --quiet "refs/tags/store/redis/v${V}"; then
	echo "tag store/redis/v${V} already exists" >&2
	exit 1
fi

git tag -a "v${V}" -m "github.com/boxgo/session v${V}"
git tag -a "store/redis/v${V}" -m "github.com/boxgo/session/store/redis v${V}"
echo "Created v${V} and store/redis/v${V}. Push with:"
echo "  git push origin v${V} store/redis/v${V}"
