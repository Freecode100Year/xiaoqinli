# Stage 1: Build xiaoqinli Go binary
FROM golang:1.23-bookworm AS builder
WORKDIR /app
COPY go.mod go.sum ./
COPY main.go ./
COPY ast/ ./ast/
COPY check/ ./check/
COPY codegen/ ./codegen/
COPY compiler/ ./compiler/
COPY evolution/ ./evolution/
COPY remedy/ ./remedy/
COPY server/ ./server/
COPY vfs/ ./vfs/
COPY skills/ ./skills/
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o xql .

# Stage 2: Sandboxed environment with Python, Node.js, and Go compilers
FROM golang:1.23-bookworm
WORKDIR /workspace

# Install Python and Node.js
RUN apt-get update && apt-get install -y --no-install-recommends \
    python3 \
    python3-pip \
    curl \
    ca-certificates \
    gnupg \
    && curl -fsSL https://deb.nodesource.com/setup_20.x | bash - \
    && apt-get install -y nodejs \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*

# Install global linters and tree-sitter CLI
RUN npm install -g eslint stylelint tree-sitter-cli

# Copy xql compiler binary
COPY --from=builder /app/xql /usr/local/bin/xql

# Expose HTTP MCP Server Port
EXPOSE 8080

# Default cmd runs the HTTP MCP server
ENTRYPOINT ["xql"]
CMD ["http", ":8080"]
