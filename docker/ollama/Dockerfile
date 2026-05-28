# ollama/ollama:0.9.6
FROM ollama/ollama@sha256:f478761c18fea69b1624e095bce0f8aab06825d09ccabcd0f88828db0df185ce

ENV HOME=/home/ollama \
    OLLAMA_HOST=0.0.0.0:11434 \
    OLLAMA_MODELS=/models

RUN mkdir -p /home/ollama /models && chown -R 10001:10001 /home/ollama /models

USER 10001:10001
EXPOSE 11434