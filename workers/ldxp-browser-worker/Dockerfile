FROM mcr.microsoft.com/playwright:latest
WORKDIR /app
COPY package.json tsconfig.json bun.lock ./
ENV BUN_INSTALL_CACHE_DIR=/tmp/bun-install-cache
RUN apt-get update && apt-get install -y --no-install-recommends unzip && rm -rf /var/lib/apt/lists/* && \
    curl -fsSL https://bun.sh/install | bash -s -- bun-v1.2.0 && \
    mv /root/.bun/bin/bun /usr/local/bin/bun && \
    rm -rf /root/.bun && \
    bun install --frozen-lockfile --no-progress
COPY src ./src
COPY tests ./tests
RUN bun run build
ENV NODE_ENV=production
CMD ["node", "dist/src/index.js"]
