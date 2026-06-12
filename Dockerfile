# golang:1.24.3-bookworm
FROM golang@sha256:29d97266c1d341b7424e2f5085440b74654ae0b61ecdba206bc12d6264844e21 AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN mkdir -p /out /out-data \
    && ./webui/build-css.sh \
    && CGO_ENABLED=1 GOOS=linux go build -tags notesseract -trimpath -ldflags="-s -w -X github.com/Yeti47/frozenfortress/frozenfortress/core/ccc.AppVersion=1.1.2" -o /out/ffwebui ./webui \
    && CGO_ENABLED=1 GOOS=linux go build -tags notesseract -trimpath -ldflags="-s -w -X github.com/Yeti47/frozenfortress/frozenfortress/core/ccc.AppVersion=1.1.2" -o /out/ffcli ./cli \
    && cp -r webui/views /out/views \
    && cp -r webui/img /out/img \
    && cp -r webui/static /out/static

# debian:bookworm-slim
FROM debian@sha256:0104b334637a5f19aa9c983a91b54c89887c0984081f2068983107a6f6c21eeb

RUN groupadd --system --gid 65532 nonroot \
    && useradd --system --no-create-home --gid 65532 --uid 65532 nonroot

WORKDIR /app

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