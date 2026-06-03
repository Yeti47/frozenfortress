# golang:1.24.3-bookworm
FROM golang@sha256:29d97266c1d341b7424e2f5085440b74654ae0b61ecdba206bc12d6264844e21 AS cert-builder

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