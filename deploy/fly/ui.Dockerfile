ARG GO_VERSION
ARG NODE_VERSION
ARG PNPM_VERSION
FROM golang:${GO_VERSION}-bookworm AS gobuild
WORKDIR /work/gobuild
COPY go.mod go.sum ./
RUN go mod download && go mod verify
COPY . .
# Server-mode API (optional but included so the container is self-contained)
RUN go build -v -o /gobuild/server ./cmd/server/main.go
# WASM build (mirrors `task wasm` in Taskfile.yml)
RUN GOOS=js GOARCH=wasm go build -o /gobuild/main.wasm ./cmd/wasm

FROM node:${NODE_VERSION}-bookworm AS nodebuild
WORKDIR /work/nodebuild
COPY ui/ ./
# git-rev-sync in vite.config.ts needs .git
COPY .git/ ./.git/
COPY --from=gobuild /gobuild/main.wasm packages/web/public/main.wasm
# wasm_exec.js must match the Go toolchain that built main.wasm
COPY --from=gobuild /usr/local/go/lib/wasm/wasm_exec.js packages/web/public/wasm_exec.js
RUN npm install -g pnpm@${PNPM_VERSION}
RUN pnpm install --frozen-lockfile
RUN pnpm --filter @gcsim/web build

FROM nginx:bookworm AS ngninxbuild
WORKDIR /work/ngninxbuild
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates \
    && rm -rf /var/lib/apt/lists/*
COPY --from=nodebuild /work/nodebuild/packages/web/dist /usr/share/nginx/html
COPY --from=gobuild /gobuild/server /out/server
COPY deploy/nginx.conf /etc/nginx/conf.d/default.conf
COPY deploy/start.sh /out/start.sh
RUN chmod +x /out/start.sh
EXPOSE 8080
CMD ["/out/start.sh"]
