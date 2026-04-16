# 自定义 Fork 长期维护工作流

这份文档用于固定你当前仓库的长期维护方式，让你可以在本地持续定制 `sub2api`，同时还能跟进原作者更新，并把你自己的版本部署到服务器。

## 推荐分支模型

- `upstream/main`
  原作者仓库的最新主线，只读参考。
- `main`
  你 fork 中的同步主线，尽量保持和 `upstream/main` 一致，不放自定义改动。
- `custom-prod`
  你的长期定制分支，所有品牌、支付、业务定制都放这里，服务器也部署这个分支。

## 第一次准备

当前仓库已经满足长期维护前提：

- `origin=https://github.com/1insxxxp/sub2api.git`
- `upstream=https://github.com/Wei-Shaw/sub2api.git`
- `custom-prod` 已从你的当前定制状态创建出来

以后请遵守一个原则：

- 不要把自定义改动直接做在 `main`
- 所有定制化修改都提交到 `custom-prod`

## 日常定制开发

在本地开始修改前：

```powershell
git switch custom-prod
git pull --ff-only origin custom-prod
```

开发完成后：

```powershell
git add .
git commit -m "你的定制提交说明"
git push origin custom-prod
```

## 同步原作者最新代码

### 方式一：使用脚本同步

仓库里已经提供了脚本：

```powershell
pwsh -File .\tools\sync-custom-prod.ps1
```

如果你希望同步完成后顺手把 `main` 和 `custom-prod` 一起推到自己的 fork：

```powershell
pwsh -File .\tools\sync-custom-prod.ps1 -Push
```

脚本做的事情是：

1. 拉取 `origin` 和 `upstream` 最新代码
2. 切到 `main`
3. 用 `upstream/main` 快进更新本地 `main`
4. 切到 `custom-prod`
5. 把新的 `main` 合并进 `custom-prod`
6. 如果传了 `-Push`，再把两个分支一起推到 `origin`

### 方式二：手动同步

```powershell
git fetch origin --prune
git fetch upstream --prune

git switch main
git merge --ff-only upstream/main
git push origin main

git switch custom-prod
git merge main
git push origin custom-prod
```

## 发生冲突时怎么处理

如果原作者更新到了你改过的同一块代码，`git merge main` 可能会冲突。这是正常的。

处理顺序：

1. 先留在 `custom-prod`
2. 手工解决冲突文件
3. 跑你能跑的测试或构建
4. `git add` 冲突文件
5. `git commit`
6. `git push origin custom-prod`

建议不要在服务器上解决这种冲突，最好始终在本地处理完，再部署。

## 部署到服务器

### 重要提醒

仓库原有的 `deploy/docker-compose.local.yml` 默认使用官方镜像：

```yaml
image: weishaw/sub2api:latest
```

这意味着如果你直接用原文件部署，服务器跑到的会是官方镜像，而不是你 fork 里的定制代码。

如果你要部署自己的定制版本，请使用仓库新增的：

- `deploy/docker-compose.custom.yml`

它会覆盖默认镜像配置，改为直接从当前仓库源码构建镜像。

### Docker 部署你的定制版本

服务器首次部署建议：

```bash
git clone https://github.com/1insxxxp/sub2api.git
cd sub2api
git checkout custom-prod

cd deploy
cp .env.example .env
# 按你的实际情况修改 .env

docker compose -f docker-compose.local.yml -f docker-compose.custom.yml up -d --build
```

后续更新部署：

```bash
cd /path/to/sub2api
git fetch origin --prune
git checkout custom-prod
git pull --ff-only origin custom-prod

cd deploy
docker compose -f docker-compose.local.yml -f docker-compose.custom.yml up -d --build
```

查看日志：

```bash
cd /path/to/sub2api/deploy
docker compose -f docker-compose.local.yml -f docker-compose.custom.yml logs -f sub2api
```

### 源码部署你的定制版本

如果你的服务器不是 Docker 部署，而是源码编译部署，可以使用：

```bash
cd /path/to/sub2api
git fetch origin --prune
git checkout custom-prod
git pull --ff-only origin custom-prod

pnpm --dir frontend install --frozen-lockfile
pnpm --dir frontend run build

cd backend
go build -tags embed -o sub2api ./cmd/server
```

然后按你服务器现有的 systemd、supervisor 或手动启动方式重启服务。

### 推荐部署策略

如果你是长期维护自己的定制版，推荐优先使用：

- `custom-prod` 作为唯一部署分支
- `deploy/docker-compose.local.yml + deploy/docker-compose.custom.yml` 作为 Docker 部署入口

这样你在服务器上执行的永远是你 fork 中 `custom-prod` 的代码，而不是原作者的公共镜像。

## 建议的长期习惯

- 本地开发只在 `custom-prod`
- `main` 只承担同步上游，不放自定义提交
- 每次部署前先把本地 `custom-prod` 推到 `origin`
- 服务器只拉 `origin/custom-prod`
- 不在服务器上直接改源码

## 一套最常用的操作顺序

### 1. 同步原作者更新

```powershell
pwsh -File .\tools\sync-custom-prod.ps1 -Push
```

### 2. 在本地继续你的定制

```powershell
git switch custom-prod
# 修改代码
git add .
git commit -m "你的新定制"
git push origin custom-prod
```

### 3. 服务器部署最新定制版本

```bash
cd /path/to/sub2api
git fetch origin --prune
git checkout custom-prod
git pull --ff-only origin custom-prod
cd deploy
docker compose -f docker-compose.local.yml -f docker-compose.custom.yml up -d --build
```
