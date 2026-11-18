# golang:1.24.3-bookworm
FROM golang@sha256:29d97266c1d341b7424e2f5085440b74654ae0b61ecdba206bc12d6264844e21 AS cert-builder

WORKDIR /src
COPY cert-bootstrap.go ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/frozenfortress-cert-bootstrap ./cert-bootstrap.go

# nginx:1.30.7-alpine
FROM nginx@sha256:bef154735288bc38d99fac41ae63730684bcc661503673a04a0f9bc526ba8598

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