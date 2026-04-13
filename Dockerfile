FROM node:22-alpine AS frontend-build
WORKDIR /app/frontend

COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci

COPY frontend/ ./
COPY contracts/ /app/contracts/
RUN npm run generate:types && npm run build

FROM golang:1.26.2-alpine AS backend-build
WORKDIR /app/backend

COPY backend/go.mod backend/go.sum ./
RUN go mod download

COPY backend/ ./
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -buildvcs=false -o /out/dnswatcher-api ./cmd/dnswatcher-api

FROM alpine:3.22
WORKDIR /app

RUN addgroup -S dnswatcher && adduser -S dnswatcher -G dnswatcher

COPY --from=backend-build /out/dnswatcher-api /app/dnswatcher-api
COPY --from=frontend-build /app/frontend/dist /app/frontend/dist

ENV HOST=0.0.0.0
ENV PORT=8080
ENV DNSWATCHER_STATIC_DIR=/app/frontend/dist
ENV DNSWATCHER_RATE_LIMIT_PER_MINUTE=20
ENV DNSWATCHER_RATE_LIMIT_BURST=5
ENV DNSWATCHER_MAX_CONCURRENT_TRACES=8
ENV DNSWATCHER_READ_HEADER_TIMEOUT=5s
ENV DNSWATCHER_READ_TIMEOUT=10s
ENV DNSWATCHER_WRITE_TIMEOUT=30s
ENV DNSWATCHER_IDLE_TIMEOUT=60s

USER dnswatcher
EXPOSE 8080

CMD ["/app/dnswatcher-api"]
