FROM node:22-alpine AS base

# Use Alpine's patched trust store instead of Node's bundled CA snapshot.
RUN apk add --no-cache 'ca-certificates>=20261020-r0'
ENV NODE_OPTIONS=--use-openssl-ca
ENV SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt

FROM base AS builder

WORKDIR /app

COPY package.json package-lock.json ./
RUN npm ci

COPY . .
ARG PUBLIC_WEB_APPVIEW_URL=""
ARG PUBLIC_APPVIEW_URL=""
ARG PUBLIC_INTERACTIVE_WIDGET=resizes-visual
ARG PUBLIC_POLAR_PRODUCT_MONTHLY=6717e6de-771b-46bb-a0bd-cda4456bd92e
ARG PUBLIC_POLAR_PRODUCT_YEARLY=4b4295d5-08d4-4d06-a72f-9fc93a999519
ENV PUBLIC_WEB_APPVIEW_URL=$PUBLIC_WEB_APPVIEW_URL
ENV PUBLIC_APPVIEW_URL=$PUBLIC_APPVIEW_URL
ENV PUBLIC_INTERACTIVE_WIDGET=$PUBLIC_INTERACTIVE_WIDGET
ENV PUBLIC_POLAR_PRODUCT_MONTHLY=$PUBLIC_POLAR_PRODUCT_MONTHLY
ENV PUBLIC_POLAR_PRODUCT_YEARLY=$PUBLIC_POLAR_PRODUCT_YEARLY
RUN npm run build

FROM base

WORKDIR /app

COPY package.json package-lock.json ./
RUN npm ci --omit=dev --omit=optional --ignore-scripts

COPY --from=builder /app/build ./build

EXPOSE 3000

ENV NODE_ENV=production
ENV PORT=3000
ENV PUBLIC_INTERACTIVE_WIDGET=resizes-visual

USER node

CMD ["node", "build/index.js"]
