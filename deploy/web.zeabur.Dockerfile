# Zeabur: ZBPACK_DOCKERFILE_PATH=deploy/web.zeabur.Dockerfile
FROM node:24-alpine AS build
WORKDIR /src
COPY apps/web/package*.json ./
RUN npm ci
COPY apps/web/ ./
ARG XINGDU_SITE_URL=https://xingdu.app
RUN XINGDU_SITE_URL="$XINGDU_SITE_URL" npm run build

FROM nginxinc/nginx-unprivileged:stable-alpine
COPY deploy/zeabur/nginx.conf.template /opt/xingdu/nginx.conf.template
COPY --chmod=755 deploy/zeabur/19-xingdu-upstream.sh /docker-entrypoint.d/19-xingdu-upstream.sh
COPY --from=build /src/dist /usr/share/nginx/html
COPY LICENSE /usr/share/licenses/xingdu/LICENSE
EXPOSE 8080
HEALTHCHECK --interval=15s --timeout=5s --start-period=10s --retries=4 CMD wget -q -O /dev/null http://127.0.0.1:8080/health/web || exit 1
