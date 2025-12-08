#!/bin/bash
# Drip Performance Profiling Script
# Runs performance test while collecting CPU and memory profiles

set -e

# Color definitions
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
RESULTS_DIR="benchmark-results"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
LOG_DIR="/tmp/drip-profile-${TIMESTAMP}"
PROFILE_DIR="${RESULTS_DIR}/profiles-${TIMESTAMP}"

# Port configuration
HTTP_TEST_PORT=3000
DRIP_SERVER_PORT=8443
PPROF_PORT=6060

# PID file
PIDS_FILE="${LOG_DIR}/pids.txt"

# Create directories
mkdir -p "$RESULTS_DIR"
mkdir -p "$LOG_DIR"
mkdir -p "$PROFILE_DIR"

# ============================================
# Helper functions
# ============================================

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1" >&2
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1" >&2
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1" >&2
}

log_step() {
    echo -e "\n${BLUE}==>${NC} $1\n" >&2
}

# Cleanup function
cleanup() {
    log_step "Cleaning up..."

    if [ -f "$PIDS_FILE" ]; then
        while read -r pid; do
            if ps -p "$pid" > /dev/null 2>&1; then
                kill "$pid" 2>/dev/null || true
            fi
        done < "$PIDS_FILE"
        rm -f "$PIDS_FILE"
    fi

    pkill -f "python.*${HTTP_TEST_PORT}" 2>/dev/null || true
    pkill -f "drip server.*${DRIP_SERVER_PORT}" 2>/dev/null || true
    pkill -f "drip http ${HTTP_TEST_PORT}" 2>/dev/null || true

    log_info "Cleanup completed"
}

trap cleanup EXIT INT TERM

# Wait for port to be available
wait_for_port() {
    local port=$1
    local max_wait=${2:-30}
    local waited=0

    while ! nc -z localhost "$port" 2>/dev/null; do
        if [ "$waited" -ge "$max_wait" ]; then
            return 1
        fi
        sleep 1
        waited=$((waited + 1))
    done
    return 0
}

# Generate test certificate
generate_test_certs() {
    log_step "Generating test TLS certificate..."

    local cert_dir="${LOG_DIR}/certs"
    mkdir -p "$cert_dir"

    openssl ecparam -name prime256v1 -genkey -noout \
        -out "${cert_dir}/server.key" >/dev/null 2>&1

    openssl req -new -x509 \
        -key "${cert_dir}/server.key" \
        -out "${cert_dir}/server.crt" \
        -days 1 \
        -subj "/C=US/ST=Test/L=Test/O=Test/CN=localhost" \
        >/dev/null 2>&1

    log_info "✓ Test certificate generated"
    echo "${cert_dir}/server.crt ${cert_dir}/server.key"
}

# Start HTTP test server
start_http_server() {
    log_step "Starting HTTP test server..."

    cat > "${LOG_DIR}/test-server.py" << 'EOF'
import http.server
import socketserver
import json
from datetime import datetime
import sys

PORT = int(sys.argv[1]) if len(sys.argv) > 1 else 3000

class TestHandler(http.server.SimpleHTTPRequestHandler):
    def do_GET(self):
        response = {
            "status": "ok",
            "timestamp": datetime.now().isoformat(),
            "message": "Test server response"
        }

        self.send_response(200)
        self.send_header('Content-type', 'application/json')
        self.end_headers()
        self.wfile.write(json.dumps(response).encode())

    def log_message(self, format, *args):
        pass

with socketserver.TCPServer(("", PORT), TestHandler) as httpd:
    print(f"Server started on port {PORT}", flush=True)
    httpd.serve_forever()
EOF

    python3 "${LOG_DIR}/test-server.py" "$HTTP_TEST_PORT" \
        > "${LOG_DIR}/http-server.log" 2>&1 &
    local pid=$!
    echo "$pid" >> "$PIDS_FILE"

    if wait_for_port "$HTTP_TEST_PORT" 10; then
        log_info "✓ HTTP test server started (PID: $pid)"
    else
        log_error "HTTP test server failed to start"
        exit 1
    fi
}

# Start Drip server with pprof
start_drip_server() {
    log_step "Starting Drip server with pprof on port $PPROF_PORT..."

    local cert_path=$1
    local key_path=$2

    ./bin/drip server \
        --port "$DRIP_SERVER_PORT" \
        --domain localhost \
        --tls-cert "$cert_path" \
        --tls-key "$key_path" \
        --pprof "$PPROF_PORT" \
        > "${LOG_DIR}/drip-server.log" 2>&1 &
    local pid=$!
    echo "$pid" >> "$PIDS_FILE"

    if wait_for_port "$DRIP_SERVER_PORT" 10; then
        log_info "✓ Drip server started (PID: $pid)"
    else
        log_error "Drip server failed to start"
        exit 1
    fi

    # Wait for pprof to be available
    if wait_for_port "$PPROF_PORT" 10; then
        log_info "✓ pprof endpoint available at http://localhost:$PPROF_PORT/debug/pprof"
    else
        log_warn "pprof endpoint not available"
    fi
}

# Start Drip client
start_drip_client() {
    log_step "Starting Drip client..."

    ./bin/drip http "$HTTP_TEST_PORT" \
        --server "localhost:${DRIP_SERVER_PORT}" \
        --insecure \
        > "${LOG_DIR}/drip-client.log" 2>&1 &
    local pid=$!
    echo "$pid" >> "$PIDS_FILE"

    sleep 3

    local tunnel_url=""
    local max_attempts=10
    local attempt=0

    while [ "$attempt" -lt "$max_attempts" ]; do
        tunnel_url=$(grep -oE 'https://[a-zA-Z0-9.-]+:[0-9]+' "${LOG_DIR}/drip-client.log" 2>/dev/null | head -1)
        if [ -n "$tunnel_url" ]; then
            break
        fi
        sleep 1
        attempt=$((attempt + 1))
    done

    if [ -z "$tunnel_url" ]; then
        log_error "Cannot get tunnel URL"
        exit 1
    fi

    log_info "✓ Drip client started (PID: $pid)"
    log_info "✓ Tunnel URL: $tunnel_url"

    echo "$tunnel_url"
}

# Collect CPU profile
collect_cpu_profile() {
    local duration=$1
    local profile_file="${PROFILE_DIR}/cpu.prof"

    log_step "Collecting CPU profile for ${duration}s..."

    curl -s "http://localhost:${PPROF_PORT}/debug/pprof/profile?seconds=${duration}" \
        -o "$profile_file"

    if [ -f "$profile_file" ] && [ -s "$profile_file" ]; then
        log_info "✓ CPU profile saved to $profile_file"
        return 0
    else
        log_error "Failed to collect CPU profile"
        return 1
    fi
}

# Collect heap profile
collect_heap_profile() {
    local profile_file="${PROFILE_DIR}/heap.prof"

    log_step "Collecting heap profile..."

    curl -s "http://localhost:${PPROF_PORT}/debug/pprof/heap" \
        -o "$profile_file"

    if [ -f "$profile_file" ] && [ -s "$profile_file" ]; then
        log_info "✓ Heap profile saved to $profile_file"
        return 0
    else
        log_error "Failed to collect heap profile"
        return 1
    fi
}

# Collect goroutine profile
collect_goroutine_profile() {
    local profile_file="${PROFILE_DIR}/goroutine.txt"

    log_step "Collecting goroutine profile..."

    curl -s "http://localhost:${PPROF_PORT}/debug/pprof/goroutine?debug=2" \
        -o "$profile_file"

    if [ -f "$profile_file" ] && [ -s "$profile_file" ]; then
        log_info "✓ Goroutine profile saved to $profile_file"
        return 0
    else
        log_error "Failed to collect goroutine profile"
        return 1
    fi
}

# Run performance test with profiling
run_profiling_test() {
    local url=$1

    log_step "Starting profiling test..."

    # Warm up
    log_info "Warming up (5s)..."
    for _ in {1..5}; do
        curl -sk "$url" > /dev/null 2>&1 || true
        sleep 1
    done

    # Start CPU profiling in background
    log_info "Starting CPU profile collection (45s)..."
    collect_cpu_profile 45 &
    local profile_pid=$!

    # Wait a moment for profiler to start
    sleep 2

    # Run load test during profiling
    log_info "Running load test (30s, 100 connections)..."
    wrk -t 8 -c 100 -d 30s --latency "$url" \
        > "${RESULTS_DIR}/profile-test-${TIMESTAMP}.txt" 2>&1

    # Wait for profiling to complete
    wait $profile_pid

    # Collect heap profile after test
    collect_heap_profile

    # Collect goroutine profile
    collect_goroutine_profile

    log_info "✓ Profiling test completed"
}

# Analyze CPU profile
analyze_cpu_profile() {
    local profile_file="${PROFILE_DIR}/cpu.prof"

    if [ ! -f "$profile_file" ]; then
        log_error "CPU profile not found"
        return 1
    fi

    log_step "Analyzing CPU profile..."

    # Generate text report
    log_info "Generating top functions report..."
    go tool pprof -text -nodecount=20 "$profile_file" \
        > "${PROFILE_DIR}/cpu-top20.txt" 2>/dev/null || true

    # Generate list report
    log_info "Generating function list..."
    go tool pprof -list=. "$profile_file" \
        > "${PROFILE_DIR}/cpu-list.txt" 2>/dev/null || true

    log_info "✓ Analysis complete"
    log_info "  - Top functions: ${PROFILE_DIR}/cpu-top20.txt"
    log_info "  - Detailed list: ${PROFILE_DIR}/cpu-list.txt"
}

# Show summary
show_summary() {
    log_step "Profiling Results Summary"

    echo ""
    echo "========================================="
    echo "  CPU Profile - Top 20 Functions"
    echo "========================================="
    if [ -f "${PROFILE_DIR}/cpu-top20.txt" ]; then
        head -30 "${PROFILE_DIR}/cpu-top20.txt"
    fi
    echo ""
    echo "========================================="
    echo "  Performance Test Results"
    echo "========================================="
    if [ -f "${RESULTS_DIR}/profile-test-${TIMESTAMP}.txt" ]; then
        grep "Requests/sec:" "${RESULTS_DIR}/profile-test-${TIMESTAMP}.txt"
        grep "Transfer/sec:" "${RESULTS_DIR}/profile-test-${TIMESTAMP}.txt"
        echo ""
        grep "50%" "${RESULTS_DIR}/profile-test-${TIMESTAMP}.txt"
        grep "99%" "${RESULTS_DIR}/profile-test-${TIMESTAMP}.txt"
    fi
    echo "========================================="
    echo ""

    log_info "Profile files location: $PROFILE_DIR"
    log_info ""
    log_info "To analyze interactively, run:"
    log_info "  go tool pprof ${PROFILE_DIR}/cpu.prof"
    log_info ""
    log_info "Web UI:"
    log_info "  go tool pprof -http=:8080 ${PROFILE_DIR}/cpu.prof"
}

# ============================================
# Main flow
# ============================================

main() {
    clear
    echo "========================================="
    echo "  Drip Performance Profiling Test"
    echo "========================================="
    echo ""

    # Check dependencies
    if ! command -v wrk &> /dev/null; then
        log_error "wrk is required (brew install wrk)"
        exit 1
    fi

    if [ ! -f "./bin/drip" ]; then
        log_error "Cannot find drip executable. Run: make build"
        exit 1
    fi

    # Generate certificate
    CERT_PATHS=$(generate_test_certs)
    CERT_FILE=$(echo "$CERT_PATHS" | awk '{print $1}')
    KEY_FILE=$(echo "$CERT_PATHS" | awk '{print $2}')

    # Start services
    start_http_server
    start_drip_server "$CERT_FILE" "$KEY_FILE"
    TUNNEL_URL=$(start_drip_client)

    # Verify connectivity
    log_info "Verifying connectivity..."
    if ! curl -sk --max-time 5 "$TUNNEL_URL" > /dev/null 2>&1; then
        log_error "Tunnel not accessible"
        exit 1
    fi
    log_info "✓ Tunnel connectivity OK"

    # Run profiling test
    run_profiling_test "$TUNNEL_URL"

    # Analyze profiles
    analyze_cpu_profile

    # Show summary
    show_summary

    log_step "Profiling completed!"
}

# Run main
main
