#!/bin/bash

set -euo pipefail

# English note.
ROOT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$ROOT_DIR"

# English note.
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# English note.
info() { echo -e "${BLUE}ℹ️  $1${NC}"; }
success() { echo -e "${GREEN}✅ $1${NC}"; }
warning() { echo -e "${YELLOW}⚠️  $1${NC}"; }
error() { echo -e "${RED}❌ $1${NC}"; }
note() { echo -e "${CYAN}ℹ️  $1${NC}"; }

# English note.
PIP_INDEX_URL="${PIP_INDEX_URL:-https://pypi.tuna.tsinghua.edu.cn/simple}"
GOPROXY="${GOPROXY:-https://goproxy.cn,direct}"

# English note.
ORIGINAL_PIP_INDEX_URL="${PIP_INDEX_URL:-}"
ORIGINAL_GOPROXY="${GOPROXY:-}"

# English note.
show_progress() {
    local pid=$1
    local message=$2
    local i=0
    local dots=""
    
    # English note.
    if ! kill -0 "$pid" 2>/dev/null; then
        # English note.
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
        printf "\r${BLUE}⏳ %s%s${NC}" "$message" "$dots"
        sleep 0.5
        
        # English note.
        if ! kill -0 "$pid" 2>/dev/null; then
            break
        fi
    done
    printf "\r"
}

echo ""
echo "=========================================="
echo "  CyberStrikeAI 一键部署启动脚本"
echo "=========================================="
echo ""

# English note.
echo ""
warning "⚠️  注意：此脚本将使用临时镜像源加速下载"
echo ""
info "Python pip 临时镜像源:"
echo "  ${PIP_INDEX_URL}"
info "Go Proxy 临时镜像源:"
echo "  ${GOPROXY}"
echo ""
note "这些设置仅在脚本运行期间生效，不会修改系统配置"
echo ""
sleep 1

CONFIG_FILE="$ROOT_DIR/config.yaml"
VENV_DIR="$ROOT_DIR/venv"
REQUIREMENTS_FILE="$ROOT_DIR/requirements.txt"
BINARY_NAME="cyberstrike-ai"

# English note.
if [ ! -f "$CONFIG_FILE" ]; then
    error "配置文件 config.yaml 不存在"
    info "请确保在项目根目录运行此脚本"
    exit 1
fi

# English note.
check_python() {
    if ! command -v python3 >/dev/null 2>&1; then
        error "未找到 python3"
        echo ""
        info "请先安装 Python 3.10 或更高版本："
        echo "  macOS:   brew install python3"
        echo "  Ubuntu:  sudo apt-get install python3 python3-venv"
        echo "  CentOS:  sudo yum install python3 python3-pip"
        exit 1
    fi
    
    PYTHON_VERSION=$(python3 --version 2>&1 | awk '{print $2}')
    PYTHON_MAJOR=$(echo "$PYTHON_VERSION" | cut -d. -f1)
    PYTHON_MINOR=$(echo "$PYTHON_VERSION" | cut -d. -f2)
    
    if [ "$PYTHON_MAJOR" -lt 3 ] || ([ "$PYTHON_MAJOR" -eq 3 ] && [ "$PYTHON_MINOR" -lt 10 ]); then
        error "Python 版本过低: $PYTHON_VERSION (需要 3.10+)"
        exit 1
    fi
    
    success "Python 环境检查通过: $PYTHON_VERSION"
}

# English note.
check_go() {
    if ! command -v go >/dev/null 2>&1; then
        error "未找到 Go"
        echo ""
        info "请先安装 Go 1.21 或更高版本："
        echo "  macOS:   brew install go"
        echo "  Ubuntu:  sudo apt-get install golang-go"
        echo "  CentOS:  sudo yum install golang"
        echo "  或访问:  https://go.dev/dl/"
        exit 1
    fi
    
    GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
    GO_MAJOR=$(echo "$GO_VERSION" | cut -d. -f1)
    GO_MINOR=$(echo "$GO_VERSION" | cut -d. -f2)
    
    if [ "$GO_MAJOR" -lt 1 ] || ([ "$GO_MAJOR" -eq 1 ] && [ "$GO_MINOR" -lt 21 ]); then
        error "Go 版本过低: $GO_VERSION (需要 1.21+)"
        exit 1
    fi
    
    success "Go 环境检查通过: $(go version)"
}

# English note.
setup_python_env() {
    if [ ! -d "$VENV_DIR" ]; then
        info "创建 Python 虚拟环境..."
        python3 -m venv "$VENV_DIR"
        success "虚拟环境创建完成"
    else
        info "Python 虚拟环境已存在"
    fi
    
    info "激活虚拟环境..."
    # shellcheck disable=SC1091
    source "$VENV_DIR/bin/activate"
    
    if [ -f "$REQUIREMENTS_FILE" ]; then
        echo ""
        note "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
        note "⚠️  使用临时 pip 镜像源（仅本次脚本运行有效）"
        note "   镜像地址: ${PIP_INDEX_URL}"
        note "   如需永久配置，请设置环境变量 PIP_INDEX_URL"
        note "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
        echo ""
        
        info "升级 pip..."
        pip install --index-url "$PIP_INDEX_URL" --upgrade pip >/dev/null 2>&1 || true
        
        info "安装 Python 依赖包..."
        echo ""
        
        # English note.
        PIP_LOG=$(mktemp)
        (
            set +e
            pip install --index-url "$PIP_INDEX_URL" -r "$REQUIREMENTS_FILE" >"$PIP_LOG" 2>&1
            echo $? > "${PIP_LOG}.exit"
        ) &
        PIP_PID=$!
        
        # English note.
        sleep 0.1
        
        # English note.
        if kill -0 "$PIP_PID" 2>/dev/null; then
            show_progress "$PIP_PID" "正在安装依赖包"
        else
            # English note.
            sleep 0.2
        fi
        
        # English note.
        wait "$PIP_PID" 2>/dev/null || true
        
        PIP_EXIT_CODE=0
        if [ -f "${PIP_LOG}.exit" ]; then
            PIP_EXIT_CODE=$(cat "${PIP_LOG}.exit" 2>/dev/null || echo "1")
            rm -f "${PIP_LOG}.exit" 2>/dev/null || true
        else
            # English note.
            if [ -f "$PIP_LOG" ] && grep -q -i "error\|failed\|exception" "$PIP_LOG" 2>/dev/null; then
                PIP_EXIT_CODE=1
            fi
        fi
        
        if [ $PIP_EXIT_CODE -eq 0 ]; then
            success "Python 依赖安装完成"
        else
            # English note.
            if grep -q "angr" "$PIP_LOG" && grep -q "Rust compiler\|can't find Rust" "$PIP_LOG"; then
                warning "angr 安装失败（需要 Rust 编译器）"
                echo ""
                info "angr 是可选依赖，主要用于二进制分析工具"
                info "如果需要使用 angr，请先安装 Rust："
                echo "  macOS:   curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh"
                echo "  Ubuntu:  curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh"
                echo "  或访问:  https://rustup.rs/"
                echo ""
                info "其他依赖已安装，可以继续使用（部分工具可能不可用）"
            else
                warning "部分 Python 依赖安装失败，但可以继续尝试运行"
                warning "如果遇到问题，请检查错误信息并手动安装缺失的依赖"
                # English note.
                echo ""
                info "错误详情（最后 10 行）："
                tail -n 10 "$PIP_LOG" | sed 's/^/  /'
                echo ""
            fi
        fi
        rm -f "$PIP_LOG"
    else
        warning "未找到 requirements.txt，跳过 Python 依赖安装"
    fi
}

# English note.
build_go_project() {
    echo ""
    note "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    note "⚠️  使用临时 Go Proxy（仅本次脚本运行有效）"
    note "   Proxy 地址: ${GOPROXY}"
    note "   如需永久配置，请设置环境变量 GOPROXY"
    note "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""
    
    info "下载 Go 依赖..."
    GO_DOWNLOAD_LOG=$(mktemp)
    (
        set +e
        export GOPROXY="$GOPROXY"
        go mod download >"$GO_DOWNLOAD_LOG" 2>&1
        echo $? > "${GO_DOWNLOAD_LOG}.exit"
    ) &
    GO_DOWNLOAD_PID=$!
    
    # English note.
    sleep 0.1
    
    # English note.
    if kill -0 "$GO_DOWNLOAD_PID" 2>/dev/null; then
        show_progress "$GO_DOWNLOAD_PID" "正在下载 Go 依赖"
    else
        # English note.
        sleep 0.2
    fi
    
    # English note.
    wait "$GO_DOWNLOAD_PID" 2>/dev/null || true
    
    GO_DOWNLOAD_EXIT_CODE=0
    if [ -f "${GO_DOWNLOAD_LOG}.exit" ]; then
        GO_DOWNLOAD_EXIT_CODE=$(cat "${GO_DOWNLOAD_LOG}.exit" 2>/dev/null || echo "1")
        rm -f "${GO_DOWNLOAD_LOG}.exit" 2>/dev/null || true
    else
        # English note.
        if [ -f "$GO_DOWNLOAD_LOG" ] && grep -q -i "error\|failed" "$GO_DOWNLOAD_LOG" 2>/dev/null; then
            GO_DOWNLOAD_EXIT_CODE=1
        fi
    fi
    rm -f "$GO_DOWNLOAD_LOG" 2>/dev/null || true
    
    if [ $GO_DOWNLOAD_EXIT_CODE -ne 0 ]; then
        error "Go 依赖下载失败"
        exit 1
    fi
    success "Go 依赖下载完成"
    
    info "构建项目..."
    GO_BUILD_LOG=$(mktemp)
    (
        set +e
        export GOPROXY="$GOPROXY"
        go build -o "$BINARY_NAME" cmd/server/main.go >"$GO_BUILD_LOG" 2>&1
        echo $? > "${GO_BUILD_LOG}.exit"
    ) &
    GO_BUILD_PID=$!
    
    # English note.
    sleep 0.1
    
    # English note.
    if kill -0 "$GO_BUILD_PID" 2>/dev/null; then
        show_progress "$GO_BUILD_PID" "正在构建项目"
    else
        # English note.
        sleep 0.2
    fi
    
    # English note.
    wait "$GO_BUILD_PID" 2>/dev/null || true
    
    GO_BUILD_EXIT_CODE=0
    if [ -f "${GO_BUILD_LOG}.exit" ]; then
        GO_BUILD_EXIT_CODE=$(cat "${GO_BUILD_LOG}.exit" 2>/dev/null || echo "1")
        rm -f "${GO_BUILD_LOG}.exit" 2>/dev/null || true
    else
        # English note.
        if [ -f "$GO_BUILD_LOG" ] && grep -q -i "error\|failed" "$GO_BUILD_LOG" 2>/dev/null; then
            GO_BUILD_EXIT_CODE=1
        fi
    fi
    
    if [ $GO_BUILD_EXIT_CODE -eq 0 ]; then
        success "项目构建完成: $BINARY_NAME"
        rm -f "$GO_BUILD_LOG"
    else
        error "项目构建失败"
        # English note.
        echo ""
        info "构建错误详情："
        cat "$GO_BUILD_LOG" | sed 's/^/  /'
        echo ""
        rm -f "$GO_BUILD_LOG"
        exit 1
    fi
}

# English note.
need_rebuild() {
    if [ ! -f "$BINARY_NAME" ]; then
        return 0
    fi
    
    # English note.
    if [ "$BINARY_NAME" -ot cmd/server/main.go ] || \
       [ "$BINARY_NAME" -ot go.mod ] || \
       find internal cmd -name "*.go" -newer "$BINARY_NAME" 2>/dev/null | grep -q .; then
        return 0
    fi
    
    return 1
}

# English note.
main() {
    # English note.
    info "检查运行环境..."
    check_python
    check_go
    echo ""
    
    # English note.
    info "设置 Python 环境..."
    setup_python_env
    echo ""
    
    # English note.
    if need_rebuild; then
        info "准备构建项目..."
        build_go_project
    else
        success "可执行文件已是最新，跳过构建"
    fi
    echo ""
    
    # English note.
    success "所有准备工作完成！"
    echo ""
    info "启动 CyberStrikeAI 服务器..."
    echo "=========================================="
    echo ""
    
    # English note.
    exec "./$BINARY_NAME"
}

# English note.
main
