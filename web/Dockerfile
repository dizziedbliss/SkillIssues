# Dev-only image: Vite dev server with HMR. Built from the repo root (see
# compose.yaml) so the shared ../../packages/markdown import resolves —
# a ./web-scoped context can never see packages/.
FROM node:22-alpine

WORKDIR /app/packages/markdown
COPY packages/markdown/package.json packages/markdown/package-lock.json* ./
RUN npm ci
COPY packages/markdown/ .

WORKDIR /app/web
COPY web/package.json web/package-lock.json* ./
RUN npm ci
COPY web/ .

EXPOSE 5173
CMD ["npm", "run", "dev", "--", "--host", "0.0.0.0"]
