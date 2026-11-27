ARG GOPROXY=https://goproxy.cn,direct
ARG APK_MIRROR=mirrors.aliyun.com

FROM golang:1.26-alpine AS builder

ARG GOPROXY
ARG APK_MIRROR

ENV GOPROXY=${GOPROXY} \
    CGO_ENABLED=0 \
    GOOS=linux

RUN sed -i "s/dl-cdn.alpinelinux.org/${APK_MIRROR}/g" /etc/apk/repositories && \
    apk add --no-cache ca-certificates tzdata

WORKDIR /src

# 先拷贝依赖声明文件以复用 Go 模块下载缓存层
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api

FROM alpine:3.23

ARG APK_MIRROR

RUN sed -i "s/dl-cdn.alpinelinux.org/${APK_MIRROR}/g" /etc/apk/repositories && \
    apk add --no-cache ca-certificates tzdata wget ffmpeg && \
    mkdir -p /app/data/transcode-tmp

ENV TZ=Asia/Shanghai

WORKDIR /app

COPY --from=builder /out/api /app/api
COPY configs/config.example.yaml /app/configs/config.yaml

EXPOSE 8080

ENTRYPOINT ["/app/api", "-config", "/app/configs/config.yaml"]
