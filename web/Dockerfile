ARG NODE_IMAGE=node:24-alpine
ARG WEB_RUNTIME_IMAGE=nginxinc/nginx-unprivileged:1.29-alpine
FROM ${NODE_IMAGE} AS builder
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci
COPY . .
RUN npm run build

FROM ${WEB_RUNTIME_IMAGE} AS web
COPY nginx.conf /etc/nginx/nginx.conf.template
COPY docker-entrypoint.sh /web-entrypoint.sh
COPY --from=builder /app/dist /usr/share/nginx/html
USER 101:101
EXPOSE 8080
HEALTHCHECK --interval=10s --timeout=3s --start-period=10s --retries=6 CMD wget -q -O /dev/null http://127.0.0.1:8080/web-health || exit 1
ENTRYPOINT ["/bin/sh", "/web-entrypoint.sh"]
CMD []
