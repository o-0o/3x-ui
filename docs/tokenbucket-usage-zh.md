# Xray Core Token Bucket 限速分支使用教程

这个分支用于：

- 一个入站里放多个客户端
- 每个客户端单独限速
- 不依赖 `tc`
- 不依赖 access log agent
- 节点流量不经过本地主控面板

当前推荐版本：

```bash
v3.4.1-tokenbucket.1
```

Release 地址：

```text
https://github.com/o-0o/3x-ui/releases/tag/v3.4.1-tokenbucket.1
```

## 已安装机器：保留配置更新

已经安装过 3x-ui 的机器不要重新安装，用这个更新。它只替换程序文件，不会改 `/etc/x-ui/x-ui.db`，所以端口、账号、密码、面板路径、入站、客户端、节点配置都会保留。

更新到最新版：

```bash
bash <(curl -Ls https://raw.githubusercontent.com/o-0o/3x-ui/feature/xray-core-tokenbucket-speed-limit/update.sh)
```

更新到指定版本：

```bash
bash <(curl -Ls https://raw.githubusercontent.com/o-0o/3x-ui/feature/xray-core-tokenbucket-speed-limit/update.sh) v3.4.1-tokenbucket.1
```

也可以在面板机器上直接用菜单：

```bash
x-ui update
```

这个分支里的 `x-ui update` 已经改成拉取 `o-0o/3x-ui`，不会误更新回官方原版。

## 新机器：首次安装

只有新服务器才用安装命令。

安装最新版：

```bash
bash <(curl -Ls https://raw.githubusercontent.com/o-0o/3x-ui/feature/xray-core-tokenbucket-speed-limit/install.sh)
```

安装指定版本：

```bash
bash <(curl -Ls https://raw.githubusercontent.com/o-0o/3x-ui/v3.4.1-tokenbucket.1/install.sh) v3.4.1-tokenbucket.1
```

## 更新后检查

```bash
systemctl status x-ui --no-pager
/usr/local/x-ui/x-ui setting -show
```

检查内置 Xray 文件是否存在：

```bash
ls -lh /usr/local/x-ui/bin/xray-linux-amd64
```

查看日志：

```bash
journalctl -u x-ui -n 100 --no-pager
```

## 怎么测试客户端限速

1. 在一个入站里创建多个客户端。
2. 给某个客户端设置限速，比如 `1 MB/s`。
3. 保存后重启 Xray。
4. 用这个客户端连接并测速。

注意：

- 前端显示单位是 `MB/s`。
- 数据库存储和 Xray 配置使用 `bytes/s`。
- `0` 表示不限速。
- 同一个用户打开多个连接，也会共享这个用户自己的限速桶，不会因为多开连接突破限速。

## 主控面板和节点是否都要更新

建议都更新到同一个 tokenbucket 版本。

主控面板需要这个版本来保存和下发 `speedLimit` 字段。

节点服务器需要这个版本里打过 patch 的 Xray-core，否则节点收到 `speedLimit` 也不会真正限速。

## 把官方 main 更新合并到 tokenbucket 分支

仓库使用两个远程地址：

```text
origin   = 自己的 Fork：https://github.com/o-0o/3x-ui
upstream = 官方仓库：https://github.com/MHSanaei/3x-ui
```

先确认远程地址：

```bash
git remote -v
```

如果没有 `upstream`，只需添加一次：

```bash
git remote add upstream https://github.com/MHSanaei/3x-ui.git
```

### 1. 合并前保存当前工作

```bash
git switch feature/xray-core-tokenbucket-speed-limit
git status
```

工作区必须干净。未完成的修改可以先提交，或者临时保存：

```bash
git stash push -u -m "wip before upstream sync"
```

### 2. 更新 Fork 的 main

```bash
git fetch upstream --tags
git switch main
git merge upstream/main
git push origin main
```

如果 `main` 没有自己的提交，`git merge upstream/main` 通常会直接快进。如果出现冲突，先解决冲突再提交，不要使用 `git reset --hard`。

### 3. 把 main 合并到 tokenbucket 分支

```bash
git switch feature/xray-core-tokenbucket-speed-limit
git pull --ff-only origin feature/xray-core-tokenbucket-speed-limit
git merge main
```

如果发生冲突：

```bash
git status
```

手工编辑带冲突标记的文件，确认保留官方更新和 tokenbucket 功能，然后执行：

```bash
git add <已经解决的文件>
git commit
```

如果暂时无法解决，可以安全取消本次合并：

```bash
git merge --abort
```

优先检查这些容易冲突的文件：

```text
internal/database/model/model.go
internal/web/service/xray.go
frontend/src/pages/clients/ClientFormModal.tsx
frontend/src/pages/clients/ClientBulkAddModal.tsx
frontend/src/schemas/client.ts
.github/workflows/release.yml
internal/web/runtime/remote.go
tools/xray-tokenbucket/patches/xray-core-v26.6.22-tokenbucket-speedlimit.patch
```

### 4. 合并后验证

```bash
go test ./internal/database/model ./internal/web/service ./internal/web/runtime ./internal/web/controller
npm --prefix frontend run build
```

如果官方更新了 Xray-core 相关代码或版本，还要验证 patched Xray 能构建：

```bash
GOCACHE=/tmp/go-build-xray \
GOMODCACHE=/tmp/go-mod-xray \
GOTOOLCHAIN=go1.26.4 \
tools/xray-tokenbucket/build-xray.sh
```

确认无误后推送 tokenbucket 分支：

```bash
git push origin feature/xray-core-tokenbucket-speed-limit
```

如果之前使用了 stash，在合并完成后恢复：

```bash
git stash pop
```

恢复后再次检查 `git status`，不要把无关文件混入下一次提交。

## 使用 GitHub Actions 编译 Linux amd64 Release 包

构建配置位于：

```text
.github/workflows/release.yml
```

当前 Linux 构建矩阵只保留：

```yaml
strategy:
  matrix:
    platform:
      - amd64
```

Windows job 已设置 `if: false`，所以不会产生 Windows 或其他架构的构建。

### 1. 确认分支和提交

```bash
git switch feature/xray-core-tokenbucket-speed-limit
git pull --ff-only origin feature/xray-core-tokenbucket-speed-limit
git status
git log -1 --oneline
```

确保工作区干净，并且需要发布的修改已经提交、推送。

### 2. 选择一个从未使用过的新版本号

查看已有 tag：

```bash
git tag --list 'v*-tokenbucket.*' --sort=v:refname | tail -10
```

例如当前版本是 `v3.4.1-tokenbucket.1`，下一版可以使用 `.2`：

```bash
TAG=v3.4.1-tokenbucket.2
git tag -a "$TAG" -m "$TAG"
git push origin "$TAG"
```

工作流匹配 `v*.*.*` tag。推送 tag 后，GitHub 会自动执行：

1. 构建前端资源。
2. 下载指定版本 Xray-core 并应用 tokenbucket patch。
3. 生成 protobuf 文件并编译限速版 Xray。
4. 静态编译 Linux amd64 的 3x-ui。
5. 打包为 `x-ui-linux-amd64.tar.gz`。
6. 创建 GitHub prerelease 并上传安装包。

### 3. 在 GitHub 查看构建

打开仓库：

```text
https://github.com/o-0o/3x-ui/actions
```

进入最新的 `Release 3X-UI`，确认 `build (amd64)` 的所有步骤为绿色。构建成功后，到这里查看 Release：

```text
https://github.com/o-0o/3x-ui/releases
```

Release 中应存在：

```text
x-ui-linux-amd64.tar.gz
```

也可以使用 GitHub CLI 查看：

```bash
gh run list --workflow "Release 3X-UI" --limit 5
gh release view "$TAG"
```

### 4. 手动运行与 tag 构建的区别

在 GitHub 的 Actions 页面点击 `Run workflow` 会触发 `workflow_dispatch`，它会生成 Actions Artifact，但不会执行“上传到 GitHub Release”步骤。

需要给节点使用 `update.sh` 安装时，应通过推送新 tag 构建正式 Release 包。

### 5. 构建失败时

不要覆盖或反复使用已经发布的 tag。修复代码并提交后，递增版本号重新构建：

```bash
git add <本次修复文件>
git commit -m "Fix amd64 release build"
git push origin feature/xray-core-tokenbucket-speed-limit

TAG=v3.4.1-tokenbucket.3
git tag -a "$TAG" -m "$TAG"
git push origin "$TAG"
```

构建成功后，主控面板和节点服务器使用同一个 tag 更新：

```bash
bash <(curl -Ls https://raw.githubusercontent.com/o-0o/3x-ui/feature/xray-core-tokenbucket-speed-limit/update.sh) "$TAG"
```

## 这个分支的核心文件

3x-ui 侧：

```text
internal/database/model/model.go
internal/web/service/xray.go
frontend/src/pages/clients/ClientFormModal.tsx
frontend/src/pages/clients/ClientBulkAddModal.tsx
```

Xray-core patch：

```text
tools/xray-tokenbucket/patches/xray-core-v26.6.22-tokenbucket-speedlimit.patch
```

构建 patched Xray：

```text
tools/xray-tokenbucket/build-xray.sh
```

维护说明：

```text
docs/tokenbucket-maintenance.md
```

## 重要提醒

已安装机器不要用 `install.sh` 当更新命令。

正确更新是：

```bash
bash <(curl -Ls https://raw.githubusercontent.com/o-0o/3x-ui/feature/xray-core-tokenbucket-speed-limit/update.sh) v3.4.1-tokenbucket.1
```

这样不会重置端口、账号、密码、WebBasePath 或入站配置。
