FROM golang:1.25-alpine AS builder
LABEL stage=gobuilder \
      maintainer=https://github.com/HSn0918/tinyredis

ARG GOEXPERIMENT=greenteagc
ENV GOEXPERIMENT=${GOEXPERIMENT}
ENV CGO_ENABLED=0
ENV GOPROXY=https://goproxy.cn,direct

WORKDIR /build

COPY . .

RUN go mod tidy && go build -o ./tiny-redis ./cmd/tinyredis
RUN go get github.com/holys/redis-cli && go install github.com/holys/redis-cli


FROM alpine:latest

ENV TZ=Asia/Shanghai

# 使用国内镜像源加速apk包的安装（注释中可根据需要使用）
# RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.tuna.tsinghua.edu.cn/g' /etc/apk/repositories

RUN apk add --no-cache ca-certificates tzdata && \
    update-ca-certificates

VOLUME /data
WORKDIR /app

COPY --from=builder /build/tiny-redis /app/tiny-redis
COPY --from=builder /go/bin/redis-cli /app/redis-cli

EXPOSE 6379

ENTRYPOINT ["./tiny-redis"]
