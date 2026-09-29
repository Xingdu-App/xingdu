FROM node:24-alpine AS build
WORKDIR /src
COPY apps/web/package*.json ./
RUN npm ci
COPY apps/web/ ./
ARG VITE_GA_MEASUREMENT_ID
ARG XINGDU_SITE_URL=https://xingdu.app
RUN XINGDU_SITE_URL="$XINGDU_SITE_URL" npm run build

FROM nginxinc/nginx-unprivileged:stable-alpine
COPY deploy/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /src/dist /usr/share/nginx/html
COPY LICENSE /usr/share/licenses/xingdu/LICENSE
