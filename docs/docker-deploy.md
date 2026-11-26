# Docker 部署说明

## 1. 组件编排清单

`docker/docker-compose.yml` 设置项目名为 `simple_bilibili`，所有容器、自建镜像、数据卷与网络均统一以 `simple_bilibili` 开头，按精简原则编排 7 个服务：

| 服务名 | 镜像 / 构建文件 | 端口映射 | 职责说明 |
| :--- | :--- | :--- | :--- |
| `mysql` | `mysql:8.4` | `3306:3306` | 业务数据库持久化，配置健康检查与持久化卷 `mysql_data` |
| `redis` | `redis:7.4-alpine` | `6379:6379` | 缓存、限流与推荐召回池，开启 AOF 持久化卷 `redis_data` |
| `rmqnamesrv` | `apache/rocketmq:5.3.4` | `9876:9876` | RocketMQ NameServer，JVM 堆内存限制为 256MB |
| `rmqbroker` | `apache/rocketmq:5.3.4` | `10911:10911`、`10909:10909` | RocketMQ Broker，挂载 `docker/rocketmq/broker.conf`，关闭自动建 Topic，JVM 堆内存限制为 512MB |
| `rmq-topic-init` | `apache/rocketmq:5.3.4` | 无 | 一次性初始化服务（`restart: "no"`），在 `rmqbroker` 健康后执行 `docker/rocketmq/init-topics.sh` 自动创建全部业务 Topic，完成后正常退出 |
| `backend` | `docker/backend.Dockerfile` | `8080:8080` | 单 Go 后端服务，等待 MySQL、Redis 健康及 `rmq-topic-init` 执行成功后启动，通过 `BILIBILI_*` 环境变量覆盖容器内连接地址 |
| `frontend` | `docker/frontend.Dockerfile` | `8088:80` | Nginx 托管前端构建产物，挂载 `docker/nginx/default.conf` 处理 SPA 路由回退及 `/api/`、`/healthz`、`/ws/` 反向代理 |

## 2. Dockerfile 构建代理与缓存策略

### 2.1 Go 后端（`docker/backend.Dockerfile`）
- 采用两阶段构建（`golang:1.26-alpine` 编译 + `alpine:3.23` 运行），最终镜像不包含 Go 编译工具链与源码。
- 通过构建参数 `ARG GOPROXY=https://goproxy.cn,direct` 与 `ARG APK_MIRROR=mirrors.aliyun.com` 配置 Go 模块代理与 Alpine 镜像源。
- 先拷贝 `go.mod` 与 `go.sum` 执行 `go mod download`，再拷贝业务代码执行 `go build`，源码变更时复用模块下载缓存层。

### 2.2 前端 Nginx（`docker/frontend.Dockerfile`）
- 采用两阶段构建（`node:24-alpine` 打包 + `nginx:1.28-alpine` 静态托管）。
- 通过构建参数 `ARG NPM_REGISTRY=https://registry.npmmirror.com` 配置 npm 代理镜像源。
- 先拷贝 `package.json` 与 `package-lock.json` 执行 `npm ci`，再拷贝源码执行 `npm run build` 产出 `dist/`。

## 3. RocketMQ Topic 自动创建机制

`rmqbroker` 在 `docker/rocketmq/broker.conf` 中设置 `autoCreateTopicEnable = false`，由专门的 `rmq-topic-init` 容器负责在启动链路中显式创建 Topic：
1. `rmq-topic-init` 依赖 `rmqbroker` 健康检查通过后启动。
2. `docker/rocketmq/init-topics.sh` 先轮询 `mqadmin clusterList` 确认 Broker 已注册到 `rmqnamesrv:9876`。
3. 依次通过 `mqadmin updateTopic` 幂等创建 7 个业务 Topic（读写队列各 4 个）：`video_transcode_topic`、`video_stat_sync_topic`、`danmaku_persist_topic`、`user_behavior_topic`、`watch_history_sync_topic`、`dynamic_fanout_topic`、`notification_topic`。
4. `backend` 通过 `depends_on` 的 `condition: service_completed_successfully` 等待 `rmq-topic-init` 成功退出后再启动。

## 4. 启动与停止命令

在仓库根目录执行：

```bash
# 构建并后台启动全部服务
docker compose -f docker/docker-compose.yml up -d --build

# 查看各容器健康状态与 Topic 初始化退出状态
docker compose -f docker/docker-compose.yml ps -a

# 查看 Topic 自动创建日志
docker compose -f docker/docker-compose.yml logs rmq-topic-init

# 停止并清理容器
docker compose -f docker/docker-compose.yml down
```
