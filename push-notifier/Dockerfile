FROM node:22-alpine

WORKDIR /app
COPY package.json pnpm-lock.yaml ./
RUN corepack enable && corepack prepare pnpm@9.15.9 --activate && pnpm install --frozen-lockfile --prod
COPY src ./src

ENV PUSH_ADDR=0.0.0.0:8091
EXPOSE 8091
VOLUME ["/app/data"]

CMD ["node", "src/index.mjs"]
