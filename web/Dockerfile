# Dev-only image: Vite dev server with HMR. Built from the repo root (see
# compose.yaml), which is also the npm workspace root (web, desktop,
# packages/*), so @skillissues/markdown and friends resolve as real
# workspace symlinks under node_modules — a ./web-scoped context can never
# see packages/.
FROM node:22-alpine@sha256:0a7108bf6c7bf5de370ffb1a3ed6be93d405b43ff159f681a8d18c0e2bc2e402

# Alpine's bundled npm has a known bug where `npm ci` hard-fails on an
# unrelated optionalDependency's platform mismatch once enough workspace
# members (desktop's electron-builder, here) pull in enough platform-
# specific optional deps, instead of skipping it. See root Dockerfile.
RUN npm install -g npm@11

WORKDIR /app
COPY package.json package-lock.json ./
COPY web/package.json web/
COPY desktop/package.json desktop/
COPY packages/markdown/package.json packages/markdown/
COPY packages/api-client/package.json packages/api-client/
COPY packages/ui/package.json packages/ui/
RUN npm ci

COPY packages/ packages/
COPY web/ web/
WORKDIR /app/web

EXPOSE 5173
CMD ["npm", "run", "dev", "--", "--host", "0.0.0.0"]
