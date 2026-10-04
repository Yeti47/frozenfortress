# golang:1.26.8-bookworm
FROM golang@sha256:a688600ca24f8a4d3ca77f95b0dd40704a9fc787c826660eb7ba0b641b8b175d AS cert-builder

WORKDIR /src
COPY cert-bootstrap.go ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/frozenfortress-cert-bootstrap ./cert-bootstrap.go

# nginx:1.27.5-alpine
FROM nginx@sha256:65645c7bb6a0661892a8b03b89d0743208a18dd2f3f17a54ef4b76fb8e2f2a10

COPY nginx.conf /etc/nginx/nginx.conf
COPY entrypoint.sh /usr/local/bin/frozenfortress-nginx-entrypoint.sh
COPY --from=cert-builder /out/frozenfortress-cert-bootstrap /usr/local/bin/frozenfortress-cert-bootstrap

RUN mkdir -p /data/certs /var/cache/nginx /var/run /tmp/nginx \
    && chown -R nginx:nginx /data /var/cache/nginx /var/run /tmp/nginx /usr/local/bin/frozenfortress-nginx-entrypoint.sh /usr/local/bin/frozenfortress-cert-bootstrap \
    && chmod 0755 /usr/local/bin/frozenfortress-nginx-entrypoint.sh /usr/local/bin/frozenfortress-cert-bootstrap

USER nginx:nginx
EXPOSE 8443

ENTRYPOINT ["/usr/local/bin/frozenfortress-nginx-entrypoint.sh"]
CMD ["nginx", "-g", "daemon off;"]