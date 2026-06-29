FROM mcr.microsoft.com/playwright:v1.61.1-jammy
WORKDIR /app
COPY package.json package-lock.json tsconfig.json ./
# Pin the image to package-lock playwright version so bundled browsers match npm ci.
RUN npm ci
COPY src ./src
COPY tests ./tests
RUN npm run build
ENV NODE_ENV=production
CMD ["node", "dist/src/index.js"]
