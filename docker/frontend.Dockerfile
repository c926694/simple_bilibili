ARG NPM_REGISTRY=https://registry.npmmirror.com

FROM node:24-alpine AS builder

ARG NPM_REGISTRY

WORKDIR /src

RUN npm config set registry "${NPM_REGISTRY}"

# 先拷贝依赖锁定文件以复用 npm 依赖缓存层
COPY package.json package-lock.json ./
RUN npm ci

COPY . .
RUN npm run build

FROM nginx:1.28-alpine

COPY --from=builder /src/dist /usr/share/nginx/html

EXPOSE 80

CMD ["nginx", "-g", "daemon off;"]
