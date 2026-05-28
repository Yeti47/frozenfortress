# golang:1.24.3-bookworm
FROM golang@sha256:29d97266c1d341b7424e2f5085440b74654ae0b61ecdba206bc12d6264844e21 AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN mkdir -p /out /out-data \
    && CGO_ENABLED=1 GOOS=linux go build -tags notesseract -trimpath -ldflags="-s -w" -o /out/ffwebui ./webui \
    && cp -r webui/views /out/views \
    && cp -r webui/img /out/img \
    && if [ -d webui/static ]; then cp -r webui/static /out/static; fi

# gcr.io/distroless/base-debian12:nonroot
FROM gcr.io/distroless/base-debian12@sha256:7a75a36f4bec82a7542c64195e402907486f9a4dd2f8797a976aa0cf31cfb470

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