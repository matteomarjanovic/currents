FROM node:22-alpine AS builder

WORKDIR /app

COPY package.json package-lock.json ./
RUN npm ci

COPY . .
ARG PUBLIC_WEB_APPVIEW_URL=""
ARG PUBLIC_INTERACTIVE_WIDGET=resizes-visual
ENV PUBLIC_WEB_APPVIEW_URL=$PUBLIC_WEB_APPVIEW_URL
ENV PUBLIC_INTERACTIVE_WIDGET=$PUBLIC_INTERACTIVE_WIDGET
RUN npm run build

FROM node:22-alpine

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
