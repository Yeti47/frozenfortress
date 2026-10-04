# golang:1.26.8-bookworm
FROM golang@sha256:a688600ca24f8a4d3ca77f95b0dd40704a9fc787c826660eb7ba0b641b8b175d AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG APP_VERSION=dev

RUN mkdir -p /out /out-data \
    && ./webui/build-css.sh \
    && CGO_ENABLED=1 GOOS=linux go build -tags notesseract -trimpath -ldflags="-s -w -X github.com/Yeti47/frozenfortress/frozenfortress/core/ccc.AppVersion=${APP_VERSION}" -o /out/ffwebui ./webui \
    && CGO_ENABLED=1 GOOS=linux go build -tags notesseract -trimpath -ldflags="-s -w -X github.com/Yeti47/frozenfortress/frozenfortress/core/ccc.AppVersion=${APP_VERSION}" -o /out/ffcli ./cli \
    && cp -r webui/views /out/views \
    && cp -r webui/img /out/img \
    && cp -r webui/static /out/static

# debian:bookworm-slim
FROM debian@sha256:0104b334637a5f19aa9c983a91b54c89887c0984081f2068983107a6f6c21eeb

RUN groupadd --system --gid 65532 nonroot \
    && useradd --system --no-create-home --gid 65532 --uid 65532 nonroot

WORKDIR /app

# The slim base has no CA bundle; outbound HTTPS (e.g. the GitHub update check) needs it.
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --chown=nonroot:nonroot --from=builder /out/ /app/
COPY --chown=nonroot:nonroot --from=builder /out-data/ /data/

ENV FF_DATABASE_PATH=/data/frozenfortress.db \
    FF_KEY_DIR=/data/keys \
    FF_BACKUP_DIRECTORY=/data/backups \
    FF_REDIS_ADDRESS=redis:6379 \
    FF_WEB_UI_PORT=8080 \
    FF_OCR_PROVIDER=ollama-tesseract \
    FF_OCR_OLLAMA_URL=http://ollama:11434

USER nonroot:nonroot
EXPOSE 8080

CMD ["/app/ffwebui"]