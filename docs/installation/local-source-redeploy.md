# 本地源码镜像部署与历史镜像清理

本文档用于统一本机 `new-api` 的 Docker 使用方式，目标是：

- 只保留并使用当前仓库 `D:\services\new-api-code` 构建出来的本地镜像
- 删除本机旧的 `newapi` 历史镜像，避免冲突和误用
- 后续本地部署只围绕一个镜像名工作：`newapi-local:from-source`

## 最终约定

后续请统一使用下面这组容器与镜像命名：

- 应用镜像：`newapi-local:from-source`
- 应用容器：`newapi-app`
- PostgreSQL 容器：`newapi-postgres`
- Redis 容器：`newapi-redis`
- Docker 网络：`new-api_default`

后续应用容器连接地址统一使用：

- PostgreSQL：`newapi-postgres`
- Redis：`newapi-redis`

不要再使用：

- `newapi-local:stable`
- `ghcr.io/calcium-ion/new-api:v0.6.0.13`
- `calciumion/new-api:latest`
- 数据库地址 `postgres`
- Redis 地址 `redis`

## 当前方案说明

当前本地环境中，标准多阶段 `docker build` 可能会因为拉取基础镜像失败而不稳定，因此这里采用一套更稳的本地方案：

1. 在当前仓库里编译后端二进制
2. 使用本地运行 Dockerfile `Dockerfile.local` 构建应用镜像
3. 用固定容器名重新部署 PostgreSQL、Redis 和应用

这套方式的好处是：

- 完全围绕当前仓库工作
- 不依赖历史 `newapi` 运行镜像
- 容器命名和连接关系清晰
- 方便后续清理旧镜像

## 前置条件

执行前请确认：

- Docker Desktop 已启动
- 当前仓库前端正式构建产物已存在
- 本地网络 `new-api_default` 已存在
- 数据目录仍保留

检查命令：

```powershell
docker network ls
docker images --format "table {{.Repository}}\t{{.Tag}}\t{{.ID}}\t{{.Size}}"
Test-Path D:\services\new-api-code\web\default\dist\index.html
Test-Path D:\services\new-api-code\web\classic\dist\index.html
Test-Path D:\services\new-api\data\new-api
Test-Path D:\services\new-api\data\postgres
Test-Path D:\services\new-api\data\redis
```

## 第一步：构建前端正式产物

如果前端目录里还是开发占位页，先重新构建前端。

```powershell
cd D:\services\new-api-code\web\default
bun run build

cd D:\services\new-api-code\web\classic
bun run build
```

构建成功后，`dist/index.html` 不应再是 `Use the frontend development server.`。

## 第二步：编译当前源码后端

```powershell
cd D:\services\new-api-code
New-Item -ItemType Directory -Force -Path .\.codex-tmp | Out-Null

docker run --rm `
  -v D:\services\new-api-code:/src `
  -w /src `
  docker.m.daocloud.io/library/golang:1.26.1-alpine `
  /usr/local/go/bin/go build -ldflags "-s -w" -o .codex-tmp/one-api
```

输出文件：

```text
D:\services\new-api-code\.codex-tmp\one-api
```

## 第三步：构建唯一应用镜像

项目根目录已经提供了 [Dockerfile.local](/D:/services/new-api-code/Dockerfile.local)。

执行：

```powershell
cd D:\services\new-api-code
docker build -f Dockerfile.local -t newapi-local:from-source .
```

构建成功后检查：

```powershell
docker images --format "table {{.Repository}}\t{{.Tag}}\t{{.ID}}\t{{.Size}}" | findstr /I "newapi-local"
```

## 第四步：重新部署后端相关容器

先删除旧容器：

```powershell
docker rm -f newapi-app newapi-postgres newapi-redis
```

启动 PostgreSQL：

```powershell
docker run -d `
  --name newapi-postgres `
  --restart always `
  --network new-api_default `
  -p 5432:5432 `
  -v D:\services\new-api\data\postgres:/var/lib/postgresql/data `
  -e POSTGRES_USER=newapi `
  -e POSTGRES_PASSWORD=i47Z63l3zWeYEnT4ciH2DjIR `
  -e POSTGRES_DB=newapi `
  postgres:16-alpine
```

启动 Redis：

```powershell
docker run -d `
  --name newapi-redis `
  --restart always `
  --network new-api_default `
  -p 6379:6379 `
  -v D:\services\new-api\data\redis:/data `
  redis:7-alpine redis-server --appendonly yes
```

启动应用：

```powershell
docker run -d `
  --name newapi-app `
  --restart always `
  --network new-api_default `
  -p 3000:3000 `
  -v D:\services\new-api\data\new-api:/data `
  -v D:\services\new-api\logs:/app/logs `
  -e POSTGRES_USER=newapi `
  -e NEWAPI_PORT=3000 `
  -e POSTGRES_DB=newapi `
  -e TZ=Asia/Shanghai `
  -e CRYPTO_SECRET=IoIcnwkrRXaOfQh5QX2aSPOxXBYNjvG98sF5sOkcyhc `
  -e SQL_DSN=postgres://newapi:i47Z63l3zWeYEnT4ciH2DjIR@newapi-postgres:5432/newapi?sslmode=disable `
  -e NEWAPI_ADMIN_USERNAME=chenjing `
  -e SESSION_SECRET=aApFd8SxeUpWs5vZCLcx8cxUaBf488BLAwhNAqRuEA4 `
  -e REDIS_PORT=6379 `
  -e POSTGRES_PASSWORD=i47Z63l3zWeYEnT4ciH2DjIR `
  -e NEWAPI_ADMIN_PASSWORD=chenjing `
  -e POSTGRES_PORT=5432 `
  -e NEWAPI_ADMIN_USER_ID=1 `
  -e REDIS_CONN_STRING=redis://newapi-redis:6379 `
  newapi-local:from-source
```

## 第五步：验证服务

检查容器：

```powershell
docker ps --filter "name=newapi-app" --filter "name=newapi-postgres" --filter "name=newapi-redis"
```

查看应用日志：

```powershell
docker logs --tail 100 newapi-app
```

检查状态接口：

```powershell
Invoke-WebRequest -UseBasicParsing http://localhost:3000/api/status | Select-Object -ExpandProperty Content
```

访问首页：

```text
http://localhost:3000
```

如果首页 HTML 里包含正式静态资源引用，例如 `/static/js/...` 和 `/static/css/...`，说明已经不是开发占位页。

## 第六步：删除历史 newapi 镜像

当 `newapi-app` 已经稳定运行在 `newapi-local:from-source` 上后，删除历史 `newapi` 镜像：

```powershell
docker rmi newapi-local:stable
docker rmi ghcr.io/calcium-ion/new-api:v0.6.0.13
docker rmi calciumion/new-api:latest
```

如果某个镜像删除时提示还有容器引用，先确认当前运行容器不是它，再删除旧容器或先保留镜像。

删除后再次检查：

```powershell
docker images --format "table {{.Repository}}\t{{.Tag}}\t{{.ID}}\t{{.Size}}" | findstr /I "newapi calciumion ghcr.io/calcium-ion"
```

理想结果应只剩：

- `newapi-local:from-source`

## 回滚方式

如果只是应用镜像需要回滚，建议重新用当前源码再构建一次，而不是恢复历史镜像。

如果必须临时回退，可以保留一个你自己确认可用的上一个本地标签，例如：

```powershell
docker tag newapi-local:from-source newapi-local:backup
```

然后用它启动：

```powershell
docker run -d ... newapi-local:backup
```

不建议再把 `newapi-local:stable` 当作长期方案。

## 常见问题

### 1. 为什么应用里要连 `newapi-postgres` 和 `newapi-redis`？

因为这是明确的容器名，不依赖额外网络别名，稳定性更高。

### 2. 为什么不再建议依赖 `newapi-local:stable`？

因为它是历史镜像，会让“当前源码构建产物”和“旧镜像运行环境”混在一起，容易产生误判。

### 3. `VERSION` 为空会怎样？

服务仍能运行，但日志和接口里版本可能显示为 `v0.0.0`。

### 4. 临时编译文件可以删吗？

可以：

```powershell
Remove-Item -Force D:\services\new-api-code\.codex-tmp\one-api
```

## 推荐的后续固定流程

以后每次本地更新部署，统一按这套顺序：

1. 前端 `bun run build`
2. 编译 `.codex-tmp/one-api`
3. `docker build -f Dockerfile.local -t newapi-local:from-source .`
4. 重启 `newapi-app`
5. 验证 `http://localhost:3000/api/status`

这样你的本地 Docker 环境就会完全围绕 `D:\services\new-api-code` 运转，不再依赖旧历史镜像。
