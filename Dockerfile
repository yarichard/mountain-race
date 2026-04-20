# syntax=docker/dockerfile:1

# ── Stage 1: Build Go backend ────────────────────────────────────────────────
FROM golang:1.26.2 AS go-builder

RUN apt-get update && apt-get install -y libeccodes-dev && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY backend/ ./backend/
WORKDIR /app/backend
RUN CGO_ENABLED=1 go build -o /app/server .

# ── Stage 2: Build frontend + assemble final image ───────────────────────────
FROM node:24-slim AS final

# System dependencies
RUN apt-get update && apt-get install -y \
    chromium \
    libeccodes-dev \
    ca-certificates \
    curl \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Build frontend
COPY frontend/ ./frontend/
WORKDIR /app/frontend
RUN npm ci && npm run build

# Assemble final layout
WORKDIR /app
COPY --from=go-builder /app/server ./server
RUN cp -r /app/frontend/out ./static

EXPOSE 8003
CMD ["./server"]
