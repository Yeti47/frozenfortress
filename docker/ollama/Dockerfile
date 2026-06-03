# ollama/ollama:0.24.0
FROM ollama/ollama@sha256:a6149234667efc71d37766d61c1a16f24c33e4cd7a0bf4125c44a7e47e2419c4

ENV HOME=/home/ollama \
    OLLAMA_HOST=0.0.0.0:11434 \
    OLLAMA_MODELS=/models \
    OLLAMA_MODEL=glm-ocr:q8_0

RUN mkdir -p /home/ollama /models && chown -R 10001:10001 /home/ollama /models

COPY --chmod=755 entrypoint.sh /entrypoint.sh

USER 10001:10001
EXPOSE 11434
ENTRYPOINT ["/entrypoint.sh"]