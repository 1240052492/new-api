#!/bin/bash

# deploy.sh - new-api 部署辅助脚本
# 功能：自动化准备 tag 和推送到 GitHub
# 使用：./scripts/deploy.sh

set -e

# 颜色输出
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo -e "${YELLOW}========================================${NC}"
echo -e "${YELLOW}new-api 部署准备脚本${NC}"
echo -e "${YELLOW}========================================${NC}"

# 检查工作目录是否干净
echo -e "\n${YELLOW}检查工作目录...${NC}"
if ! git diff-index --quiet HEAD --; then
    echo -e "${RED}❌ 错误：工作目录有未提交的更改${NC}"
    git status
    exit 1
fi

# 检查当前分支
CURRENT_BRANCH=$(git rev-parse --abbrev-ref HEAD)
echo -e "当前分支: ${GREEN}$CURRENT_BRANCH${NC}"

# 如果在 develop，则切换到 main
if [ "$CURRENT_BRANCH" != "main" ]; then
    echo -e "${YELLOW}切换到 main 分支...${NC}"
    git checkout main
    git pull origin main
fi

# 检查 main 和 develop 是否需要合并
echo -e "\n${YELLOW}检查是否需要合并 develop...${NC}"
if git diff --quiet main develop; then
    echo -e "ℹ️  main 和 develop 代码相同，跳过合并"
else
    echo -e "${YELLOW}合并 develop 到 main...${NC}"
    git merge develop --no-ff -m "Release: 合并 develop 到 main"
fi

# 创建 tag
echo -e "\n${YELLOW}准备创建版本 Tag...${NC}"
DATE=$(date +%Y%m%d)

# 检查该日期是否已有 tag
EXISTING_TAGS=$(git tag -l "v1.0.0-aiplantform-${DATE}-*" | wc -l)
VERSION_NUM=$((EXISTING_TAGS + 1))

TAG="v1.0.0-aiplantform-${DATE}-${VERSION_NUM}"

echo -e "版本 Tag: ${GREEN}$TAG${NC}"
echo -e "发布日期: ${GREEN}$DATE${NC}"
echo -e "该日期第 $VERSION_NUM 次发布"

# 验证 tag 不存在
if git rev-parse "$TAG" >/dev/null 2>&1; then
    echo -e "${RED}❌ 错误：Tag $TAG 已存在${NC}"
    exit 1
fi

# 创建 tag
echo -e "\n${YELLOW}创建带注解的 Tag...${NC}"
git tag -a "$TAG" -m "Release $TAG - new-api production deployment"

# 推送到 GitHub
echo -e "\n${YELLOW}推送代码和 Tag 到 GitHub...${NC}"
git push origin main
git push origin "$TAG"

# 最终输出
echo -e "\n${GREEN}========================================${NC}"
echo -e "${GREEN}✅ 部署准备完成！${NC}"
echo -e "${GREEN}========================================${NC}"
echo -e ""
echo -e "${GREEN}版本 Tag: $TAG${NC}"
echo -e ""
echo -e "下一步在服务器上执行:"
echo -e "  ${YELLOW}cd /opt/new-api${NC}"
echo -e "  ${YELLOW}git fetch origin main${NC}"
echo -e "  ${YELLOW}git fetch origin --tags${NC}"
echo -e "  ${YELLOW}git checkout $TAG${NC}"
echo -e "  ${YELLOW}sudo systemctl restart new-api${NC}"
echo -e ""
echo -e "验证部署:"
echo -e "  ${YELLOW}curl http://localhost:3000/api/health${NC}"
echo -e "  ${YELLOW}git describe --tags${NC}"
