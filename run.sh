#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$ROOT_DIR"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

info() { echo -e "${BLUE}[INFO] $1${NC}"; }
success() { echo -e "${GREEN}[ OK ] $1${NC}"; }
warning() { echo -e "${YELLOW}[WARN] $1${NC}"; }
error() { echo -e "${RED}[ERR ] $1${NC}"; }
note() { echo -e "${CYAN}[NOTE] $1${NC}"; }

PIP_INDEX_URL="${PIP_INDEX_URL:-https://pypi.tuna.tsinghua.edu.cn/simple}"
GOPROXY="${GOPROXY:-https://goproxy.cn,direct}"

show_progress() {
    local pid=$1
    local message=$2
    local i=0
    local dots=""

    if ! kill -0 "$pid" 2>/dev/null; then
        return 0
    fi

    while kill -0 "$pid" 2>/dev/null; do
        i=$((i + 1))
        case $((i % 4)) in
            0) dots="." ;;
            1) dots=".." ;;
            2) dots="..." ;;
            3) dots="...." ;;
        esac
        printf "\r${BLUE}[....] %s%s${NC}" "$message" "$dots"
        sleep 0.5

        if ! kill -0 "$pid" 2>/dev/null; then
            break
        fi
    done
    printf "\r"
}

echo ""
echo "=========================================="
echo "  CyberStrikeAI Launcher"
echo "=========================================="
echo ""

warning "This script may use temporary package mirrors to speed up downloads."
echo ""
info "Temporary Python package index:"
echo "  ${PIP_INDEX_URL}"
info "Temporary Go proxy:"
echo "  ${GOPROXY}"
echo ""
note "These settings apply only while this script runs and will not modify your system configuration."
echo ""
sleep 1

CONFIG_FILE="$ROOT_DIR/config.yaml"
VENV_DIR="$ROOT_DIR/venv"
REQUIREMENTS_FILE="$ROOT_DIR/requirements.txt"
BINARY_NAME="cyberstrike-ai"

if [ ! -f "$CONFIG_FILE" ]; then
    error "config.yaml was not found."
    info "Please run this script from the project root directory."
    exit 1
fi

check_python() {
    if ! command -v python3 >/dev/null 2>&1; then
        error "python3 was not found."
        echo ""
        info "Please install Python 3.10 or later:"
        echo "  macOS:   brew install python3"
        echo "  Ubuntu:  sudo apt-get install python3 python3-venv"
        echo "  CentOS:  sudo yum install python3 python3-pip"
        exit 1
    fi

    PYTHON_VERSION=$(python3 --version 2>&1 | awk '{print $2}')
    PYTHON_MAJOR=$(echo "$PYTHON_VERSION" | cut -d. -f1)
    PYTHON_MINOR=$(echo "$PYTHON_VERSION" | cut -d. -f2)

    if [ "$PYTHON_MAJOR" -lt 3 ] || ([ "$PYTHON_MAJOR" -eq 3 ] && [ "$PYTHON_MINOR" -lt 10 ]); then
        error "Python version is too old: $PYTHON_VERSION (requires 3.10+)."
        exit 1
    fi

    success "Python check passed: $PYTHON_VERSION"
}

check_go() {
    if ! command -v go >/dev/null 2>&1; then
        error "Go was not found."
        echo ""
        info "Please install Go 1.21 or later:"
        echo "  macOS:   brew install go"
        echo "  Ubuntu:  sudo apt-get install golang-go"
        echo "  CentOS:  sudo yum install golang"
        echo "  Download: https://go.dev/dl/"
        exit 1
    fi

    GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
    GO_MAJOR=$(echo "$GO_VERSION" | cut -d. -f1)
    GO_MINOR=$(echo "$GO_VERSION" | cut -d. -f2)

    if [ "$GO_MAJOR" -lt 1 ] || ([ "$GO_MAJOR" -eq 1 ] && [ "$GO_MINOR" -lt 21 ]); then
        error "Go version is too old: $GO_VERSION (requires 1.21+)."
        exit 1
    fi

    success "Go check passed: $(go version)"
}

check_cgo() {
    if ! command -v gcc >/dev/null 2>&1; then
        error "gcc was not found."
        echo ""
        info "This project uses CGO for SQLite, so a C compiler is required."
        info "Please install gcc/build tools first:"
        echo "  Ubuntu:  sudo apt-get install build-essential"
        echo "  Debian:  sudo apt-get install build-essential"
        echo "  CentOS:  sudo yum groupinstall 'Development Tools'"
        echo "  Fedora:  sudo dnf groupinstall 'Development Tools'"
        exit 1
    fi

    success "C compiler check passed: $(gcc --version | head -n 1)"
}

setup_python_env() {
    if [ ! -d "$VENV_DIR" ]; then
        info "Creating Python virtual environment..."
        python3 -m venv "$VENV_DIR"
        success "Virtual environment created."
    else
        info "Python virtual environment already exists."
    fi

    info "Activating virtual environment..."
    # shellcheck disable=SC1091
    source "$VENV_DIR/bin/activate"

    if [ -f "$REQUIREMENTS_FILE" ]; then
        echo ""
        note "------------------------------------------------------------"
        note "Using a temporary pip mirror for this run only."
        note "Mirror: ${PIP_INDEX_URL}"
        note "Set PIP_INDEX_URL if you want to override it permanently."
        note "------------------------------------------------------------"
        echo ""

        info "Upgrading pip..."
        pip install --index-url "$PIP_INDEX_URL" --upgrade pip >/dev/null 2>&1 || true

        info "Installing Python dependencies..."
        echo ""

        PIP_LOG=$(mktemp)
        (
            set +e
            pip install --index-url "$PIP_INDEX_URL" -r "$REQUIREMENTS_FILE" >"$PIP_LOG" 2>&1
            echo $? > "${PIP_LOG}.exit"
        ) &
        PIP_PID=$!

        sleep 0.1

        if kill -0 "$PIP_PID" 2>/dev/null; then
            show_progress "$PIP_PID" "Installing Python dependencies"
        else
            sleep 0.2
        fi

        wait "$PIP_PID" 2>/dev/null || true

        PIP_EXIT_CODE=0
        if [ -f "${PIP_LOG}.exit" ]; then
            PIP_EXIT_CODE=$(cat "${PIP_LOG}.exit" 2>/dev/null || echo "1")
            rm -f "${PIP_LOG}.exit" 2>/dev/null || true
        elif [ -f "$PIP_LOG" ] && grep -q -i "error\|failed\|exception" "$PIP_LOG" 2>/dev/null; then
            PIP_EXIT_CODE=1
        fi

        if [ "$PIP_EXIT_CODE" -eq 0 ]; then
            success "Python dependencies installed."
        else
            if grep -q "angr" "$PIP_LOG" && grep -q "Rust compiler\|can't find Rust" "$PIP_LOG"; then
                warning "angr could not be installed because Rust is missing."
                echo ""
                info "angr is optional and is mainly used for binary-analysis workflows."
                info "If you need angr, install Rust first:"
                echo "  macOS:   curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh"
                echo "  Ubuntu:  curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh"
                echo "  Download: https://rustup.rs/"
                echo ""
                info "Other dependencies were installed, so you can continue. Some tools may remain unavailable."
            else
                warning "Some Python dependencies failed to install, but startup may still continue."
                warning "If something breaks, review the error output and install missing packages manually."
                echo ""
                info "Last 10 lines of the pip error log:"
                tail -n 10 "$PIP_LOG" | sed 's/^/  /'
                echo ""
            fi
        fi

        rm -f "$PIP_LOG"
    else
        warning "requirements.txt was not found. Skipping Python dependency installation."
    fi
}

build_go_project() {
    echo ""
    note "------------------------------------------------------------"
    note "Using a temporary Go proxy for this run only."
    note "Proxy: ${GOPROXY}"
    note "Set GOPROXY if you want to override it permanently."
    note "------------------------------------------------------------"
    echo ""

    info "Downloading Go dependencies..."
    GO_DOWNLOAD_LOG=$(mktemp)
    (
        set +e
        export GOPROXY="$GOPROXY"
        go mod download >"$GO_DOWNLOAD_LOG" 2>&1
        echo $? > "${GO_DOWNLOAD_LOG}.exit"
    ) &
    GO_DOWNLOAD_PID=$!

    sleep 0.1

    if kill -0 "$GO_DOWNLOAD_PID" 2>/dev/null; then
        show_progress "$GO_DOWNLOAD_PID" "Downloading Go dependencies"
    else
        sleep 0.2
    fi

    wait "$GO_DOWNLOAD_PID" 2>/dev/null || true

    GO_DOWNLOAD_EXIT_CODE=0
    if [ -f "${GO_DOWNLOAD_LOG}.exit" ]; then
        GO_DOWNLOAD_EXIT_CODE=$(cat "${GO_DOWNLOAD_LOG}.exit" 2>/dev/null || echo "1")
        rm -f "${GO_DOWNLOAD_LOG}.exit" 2>/dev/null || true
    elif [ -f "$GO_DOWNLOAD_LOG" ] && grep -q -i "error\|failed" "$GO_DOWNLOAD_LOG" 2>/dev/null; then
        GO_DOWNLOAD_EXIT_CODE=1
    fi
    rm -f "$GO_DOWNLOAD_LOG" 2>/dev/null || true

    if [ "$GO_DOWNLOAD_EXIT_CODE" -ne 0 ]; then
        error "Failed to download Go dependencies."
        exit 1
    fi
    success "Go dependencies downloaded."

    info "Building project..."
    GO_BUILD_LOG=$(mktemp)
    (
        set +e
        export GOPROXY="$GOPROXY"
        go build -o "$BINARY_NAME" cmd/server/main.go >"$GO_BUILD_LOG" 2>&1
        echo $? > "${GO_BUILD_LOG}.exit"
    ) &
    GO_BUILD_PID=$!

    sleep 0.1

    if kill -0 "$GO_BUILD_PID" 2>/dev/null; then
        show_progress "$GO_BUILD_PID" "Building project"
    else
        sleep 0.2
    fi

    wait "$GO_BUILD_PID" 2>/dev/null || true

    GO_BUILD_EXIT_CODE=0
    if [ -f "${GO_BUILD_LOG}.exit" ]; then
        GO_BUILD_EXIT_CODE=$(cat "${GO_BUILD_LOG}.exit" 2>/dev/null || echo "1")
        rm -f "${GO_BUILD_LOG}.exit" 2>/dev/null || true
    elif [ -f "$GO_BUILD_LOG" ] && grep -q -i "error\|failed" "$GO_BUILD_LOG" 2>/dev/null; then
        GO_BUILD_EXIT_CODE=1
    fi

    if [ "$GO_BUILD_EXIT_CODE" -eq 0 ]; then
        success "Build completed: $BINARY_NAME"
        rm -f "$GO_BUILD_LOG"
    else
        error "Build failed."
        echo ""
        info "Build output:"
        sed 's/^/  /' "$GO_BUILD_LOG"
        echo ""
        rm -f "$GO_BUILD_LOG"
        exit 1
    fi
}

need_rebuild() {
    if [ ! -f "$BINARY_NAME" ]; then
        return 0
    fi

    if [ "$BINARY_NAME" -ot cmd/server/main.go ] || \
       [ "$BINARY_NAME" -ot go.mod ] || \
       find internal cmd -name "*.go" -newer "$BINARY_NAME" 2>/dev/null | grep -q .; then
        return 0
    fi

    return 1
}

main() {
    info "Checking runtime environment..."
    check_python
    check_go
    check_cgo
    echo ""

    info "Preparing Python environment..."
    setup_python_env
    echo ""

    if need_rebuild; then
        info "Preparing build..."
        build_go_project
    else
        success "Binary is up to date. Skipping build."
    fi
    echo ""

    success "Preparation complete."
    echo ""
    info "Starting CyberStrikeAI server..."
    echo "=========================================="
    echo ""

    exec "./$BINARY_NAME"
}

main
