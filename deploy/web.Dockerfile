FROM node:24 AS dev
RUN corepack enable
WORKDIR /app
CMD ["sh", "-c", "pnpm install --frozen-lockfile && pnpm run dev"]

FROM dev AS build
COPY web .
RUN pnpm install --frozen-lockfile && pnpm exec vite build

FROM nginx:alpine AS prod
COPY --from=build /app/dist /usr/share/nginx/html
