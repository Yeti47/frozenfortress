# Two variants of the OCR service, selected with the build target:
#   cpu (default) - only the Ollama binary and its CPU backends on a plain
#                   Ubuntu base, about 130 MB.
#   gpu           - the official Ollama image with its CUDA, MLX and Vulkan
#                   backends (about 6 GB of GPU libraries). See compose.gpu.yaml.

# ollama/ollama:0.24.0
FROM ollama/ollama@sha256:a6149234667efc71d37766d61c1a16f24c33e4cd7a0bf4125c44a7e47e2419c4 AS ollama

FROM ollama AS gpu

ENV HOME=/home/ollama \
    OLLAMA_HOST=0.0.0.0:11434 \
    OLLAMA_MODELS=/models \
    OLLAMA_MODEL=glm-ocr:q8_0

RUN mkdir -p /home/ollama /models && chown -R 10001:10001 /home/ollama /models

COPY --chmod=755 entrypoint.sh /entrypoint.sh

USER 10001:10001
EXPOSE 11434
ENTRYPOINT ["/entrypoint.sh"]

# Strip the GPU backends so the cpu stage can copy the library folder as a
# whole (a directory copy keeps the libggml-base symlinks intact).
FROM ollama AS cpu-libs
RUN rm -rf /usr/lib/ollama/cuda_* /usr/lib/ollama/mlx_* /usr/lib/ollama/vulkan /usr/lib/ollama/include

# Last stage, so a build without --target produces the CPU image.
# The CPU backends only link against libc, libstdc++ and libgcc, which the
# Ubuntu base already has. The CA bundle is needed to pull models over HTTPS.
# ubuntu:24.04
FROM ubuntu@sha256:008173c23f95b170204355c12626cb5a965d779a7e1283b09e9cffbb1bf33ca3 AS cpu
COPY --from=cpu-libs /usr/bin/ollama /usr/bin/ollama
COPY --from=cpu-libs /usr/lib/ollama /usr/lib/ollama
COPY --from=cpu-libs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

ENV HOME=/home/ollama \
    OLLAMA_HOST=0.0.0.0:11434 \
    OLLAMA_MODELS=/models \
    OLLAMA_MODEL=glm-ocr:q8_0

RUN mkdir -p /home/ollama /models && chown -R 10001:10001 /home/ollama /models

COPY --chmod=755 entrypoint.sh /entrypoint.sh

USER 10001:10001
EXPOSE 11434
ENTRYPOINT ["/entrypoint.sh"]
