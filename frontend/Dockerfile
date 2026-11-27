# Node must stay in sync with frontend/.nvmrc (used by CI).
# Base image is pinned by digest; Dependabot (docker ecosystem) proposes updates.
FROM node:26-alpine3.24@sha256:6bd9ab43c0ec0190ca7b2d193906cb8b7edf6e7f4afa43ddeccd6452710bffb5 AS base

FROM base AS deps
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci --no-audit --no-fund

FROM base AS build
WORKDIR /app
ENV NEXT_TELEMETRY_DISABLED=1
# Browser-side API base is baked in at build time; "/api/v1" = same origin via Nginx.
# The build needs neither the API nor the database (all data pages are dynamic).
ARG NEXT_PUBLIC_API_BASE_URL=/api/v1
ENV NEXT_PUBLIC_API_BASE_URL=$NEXT_PUBLIC_API_BASE_URL
COPY --from=deps /app/node_modules ./node_modules
COPY . .
RUN npm run build

FROM base AS run
WORKDIR /app
ENV NODE_ENV=production NEXT_TELEMETRY_DISABLED=1 PORT=3000 HOSTNAME=0.0.0.0
RUN addgroup -S -g 10001 app && adduser -S -D -H -u 10001 -G app app
# Files are owned by root and only readable by the app user: the server cannot rewrite its code.
COPY --from=build /app/.next/standalone ./
COPY --from=build /app/.next/static ./.next/static
COPY --from=build /app/public ./public
USER 10001:10001
EXPOSE 3000
CMD ["node", "server.js"]
