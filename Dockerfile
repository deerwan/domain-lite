# 阶段1：构建前端（pure-admin-thin）
# 用 Debian 系（glibc）而非 alpine(musl)：esbuild/rollup 等原生二进制为 glibc 链接，
# 在 alpine 上加载会失败导致 pnpm build 退出码 1
FROM node:22-bookworm AS web
WORKDIR /web
RUN npm install -g pnpm@9
# 必须带上 .npmrc（含 shamefully-hoist=true），否则 pnpm 默认严格模式不会提升
# vite-plugin-cdn-import 依赖的 vue-demi，导致 CI 构建报
# "modules: vue-demi package.json file does not exist"
COPY web/package.json web/pnpm-lock.yaml web/.npmrc ./
RUN pnpm install --frozen-lockfile
COPY web/ ./
RUN pnpm build

# 阶段2：构建后端（Go，纯静态二进制，无需 cgo）
FROM golang:1.22-alpine AS backend
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# 把前端构建产物放入 embed 目录，打包进二进制
COPY --from=web /web/dist ./internal/static/dist
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /domain-lite ./cmd/server

# 阶段3：运行时（极小镜像）
FROM alpine:3.20
WORKDIR /app
RUN apk add --no-cache ca-certificates tzdata
COPY --from=backend /domain-lite /usr/local/bin/domain-lite
ENV PORT=8080 \
    DB_PATH=/data/domain-lite.db

# 注意：JWT_SECRET 不在此设默认值，必须由运行时 -e JWT_SECRET=... 显式提供，
# 缺失时程序启动会直接失败（见 internal/config/config.go）。
VOLUME ["/data"]
EXPOSE 8080
ENTRYPOINT ["domain-lite"]
