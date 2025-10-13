#!/bin/bash

# AI Infrastructure Agent Web UI Launch Script

echo "🚀 Building AI Infrastructure Agent Web UI..."

# Build the web application
go build -o bin/web-ui cmd/web/main.go

if [ $? -ne 0 ]; then
    echo "❌ Build failed!"
    exit 1
fi

echo "✅ Build successful!"

# Set default values
PORT=${PORT:-8080}

# Create default config if it doesn't exist
if [ ! -f config.yaml ]; then
    echo "📝 Creating default configuration..."
    cat > config.yaml << EOF
server:
  port: ${PORT}
  host: "localhost"

aws:
  region: "us-west-2"

mcp:
  server_name: "ai-infrastructure-agent"
  version: "1.0.0"
EOF
fi

# Start the web UI
echo "🌐 Starting AI Infrastructure Agent Web UI on port ${PORT}..."
echo "🔗 Open: http://localhost:${PORT}"
echo ""
echo "Press Ctrl+C to stop the server"
echo ""

./bin/web-ui
