FROM mcr.microsoft.com/playwright:latest
WORKDIR /app
COPY package.json package-lock.json tsconfig.json ./
# Playwright latest currently does not support corepack prepare bun@latest;
# use the Task 11 npm fallback with package-lock for container builds.
RUN npm ci
COPY src ./src
COPY tests ./tests
RUN npm run build
ENV NODE_ENV=production
CMD ["node", "dist/src/index.js"]
