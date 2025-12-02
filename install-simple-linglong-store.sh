#!/usr/bin/env bash
set -e

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

info() { echo -e "${GREEN}[INFO]${NC} $*"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $*"; }
error() { echo -e "${RED}[ERROR]${NC} $*" >&2; }

# ========================================
# 检测发行版信息
# ========================================
detect_distro() {
    if [ ! -f /etc/os-release ]; then
        error "无法检测发行版：/etc/os-release 不存在"
        exit 1
    fi
    
    . /etc/os-release
    
    DISTRO_ID="$ID"
    DISTRO_VERSION="$VERSION_ID"
    DISTRO_NAME="${PRETTY_NAME:-$ID $VERSION_ID}"
    
    info "检测到发行版: $DISTRO_NAME"
}

# ========================================
# 检查玲珑环境是否已安装
# ========================================
check_linglong_installed() {
    if command -v ll-cli >/dev/null 2>&1; then
        local version
        version=$(ll-cli --version 2>/dev/null | head -n1 || echo "未知版本")
        info "玲珑环境已安装: $version"
        return 0
    else
        warn "玲珑环境未安装"
        return 1
    fi
}


# ========================================
# 添加 APT 软件源 (Debian/Ubuntu/Deepin/UOS/openKylin)
# ========================================
add_apt_repo() {
    local repo_path="$1"
    local repo_file="/etc/apt/sources.list.d/linglong.list"
    local repo_url="https://ci.deepin.com/repo/obs/linglong:/CI:/release/${repo_path}/"
    
    if [ -f "$repo_file" ]; then
        info "linglong 软件源已存在，跳过添加"
    else
        info "添加软件源: $repo_url"
        echo "deb [trusted=yes] ${repo_url} ./" | sudo tee "$repo_file" > /dev/null
    fi
    
    info "更新软件源..."
    sudo apt update
}

# ========================================
# 添加 DNF 软件源 (Fedora/openEuler/AnolisOS)
# ========================================
add_dnf_repo() {
    local repo_url="$1"
    
    if [ -f "/etc/yum.repos.d/linglong*.repo" ] 2>/dev/null; then
        info "linglong 软件源已存在，跳过添加"
    else
        info "添加软件源: $repo_url"
        sudo dnf config-manager addrepo --from-repofile "$repo_url" || \
        sudo dnf config-manager --add-repo "$repo_url"
        
        # openEuler 需要禁用 gpgcheck
        if [ "$DISTRO_ID" = "openEuler" ]; then
            sudo sh -c "echo gpgcheck=0 >> /etc/yum.repos.d/linglong*.repo"
        fi
    fi
    
    info "更新软件源..."
    sudo dnf update -y --refresh
}

# ========================================
# 安装玲珑环境
# ========================================
install_linglong() {
    
    case "$DISTRO_ID" in
        # ===== APT 系 =====
        deepin)
            case "$DISTRO_VERSION" in
                25)
                    add_apt_repo "Deepin_25"
                    sudo apt install -y linglong-bin linglong-installer
                    ;;
                23)
                    add_apt_repo "Deepin_23"
                    sudo apt install -y linglong-bin linglong-installer
                    ;;
                *)
                    error "不支持的 Deepin 版本: $DISTRO_VERSION"
                    exit 1
                    ;;
            esac
            ;;
        ubuntu)
            case "$DISTRO_VERSION" in
                24.04)
                    add_apt_repo "xUbuntu_24.04"
                    sudo apt install -y linglong-bin linglong-installer
                    ;;
                *)
                    error "不支持的 Ubuntu 版本: $DISTRO_VERSION (目前仅支持 24.04)"
                    exit 1
                    ;;
            esac
            ;;
        debian)
            case "$DISTRO_VERSION" in
                12)
                    add_apt_repo "Debian_12"
                    sudo apt install -y linglong-bin linglong-installer
                    ;;
                13)
                    add_apt_repo "Debian_13"
                    sudo apt install -y linglong-bin linglong-installer
                    ;;
                *)
                    error "不支持的 Debian 版本: $DISTRO_VERSION (目前仅支持 12/13)"
                    exit 1
                    ;;
            esac
            ;;
        uos)
            # UOS 20 系列统一使用 uos_1070 源
            add_apt_repo "uos_1070"
            sudo apt install -y linglong-bin linglong-installer
            ;;
        openkylin)
            case "$DISTRO_VERSION" in
                2.0)
                    add_apt_repo "openkylin_2.0"
                    sudo apt install -y linglong-bin linglong-installer
                    ;;
                *)
                    error "不支持的 openKylin 版本: $DISTRO_VERSION (目前仅支持 2.0)"
                    exit 1
                    ;;
            esac
            ;;
        
        # ===== DNF 系 =====
        fedora)
            case "$DISTRO_VERSION" in
                41)
                    add_dnf_repo "https://ci.deepin.com/repo/obs/linglong:/CI:/release/Fedora_41/linglong%3ACI%3Arelease.repo"
                    sudo dnf install -y linglong-bin linyaps-web-store-installer
                    ;;
                42)
                    add_dnf_repo "https://ci.deepin.com/repo/obs/linglong:/CI:/release/Fedora_42/linglong%3ACI%3Arelease.repo"
                    sudo dnf install -y linglong-bin linyaps-web-store-installer
                    ;;
                *)
                    error "不支持的 Fedora 版本: $DISTRO_VERSION (目前仅支持 41/42)"
                    exit 1
                    ;;
            esac
            ;;
        openEuler)
            case "$DISTRO_VERSION" in
                23.09)
                    add_dnf_repo "https://ci.deepin.com/repo/obs/linglong:/CI:/release/openEuler_23.09/linglong%3ACI%3Arelease.repo"
                    sudo dnf install -y linglong-bin linyaps-web-store-installer
                    ;;
                24.03)
                    add_dnf_repo "https://ci.deepin.com/repo/obs/linglong:/CI:/release/openEuler_24.03/linglong%3ACI%3Arelease.repo"
                    sudo dnf install -y linglong-bin linyaps-web-store-installer
                    ;;
                *)
                    error "不支持的 openEuler 版本: $DISTRO_VERSION (目前仅支持 23.09/24.03)"
                    exit 1
                    ;;
            esac
            ;;
        anolis)
            add_dnf_repo "https://ci.deepin.com/repo/obs/linglong:/CI:/release/AnolisOS_8/linglong%3ACI%3Arelease.repo"
            sudo dnf install -y linglong-bin linyaps-web-store-installer
            ;;
        
        # ===== Pacman 系 =====
        arch|manjaro|parabola)
            info "使用 pacman 安装 linyaps..."
            sudo pacman -Syu --noconfirm linyaps
            ;;
        
        *)
            error "不支持的发行版: $DISTRO_NAME"
            echo ""
            echo "目前支持的发行版:"
            echo "  APT 系: Deepin 23/25, Ubuntu 24.04, Debian 12/13, UOS 1070, openKylin 2.0"
            echo "  DNF 系: Fedora 41/42, openEuler 23.09/24.03, AnolisOS 8"
            echo "  Pacman 系: Arch, Manjaro, Parabola Linux"
            exit 1
            ;;
    esac
    
    info "玲珑环境安装完成！"
}

# ========================================
# 主流程：检查玲珑环境，没有就安装
# ========================================
ensure_linglong_installed() {
    detect_distro
    
    if check_linglong_installed; then
        info "玲珑环境检查通过"
    else
        info "开始安装玲珑环境..."
        install_linglong
        
        # 验证安装
        if check_linglong_installed; then
            info "玲珑环境安装验证通过"
        else
            error "玲珑环境安装失败，请检查日志"
            exit 1
        fi
    fi
}

# ========================================
# 主入口
# ========================================
main() {
    echo "========================================"
    echo "  简易玲珑商店安装脚本"
    echo "========================================"
    echo ""
    
    # 1. 检查玲珑环境，没有就安装
    ensure_linglong_installed
    
    echo ""
    info "玲珑环境就绪，继续安装玲珑商店..."
    echo ""
    
    # 2. 配置玲珑测试源
    info "添加玲珑商店测试源：https://cdn-linglong.odata.cc ..."
    sudo ll-cli repo add --alias=testing stable https://cdn-linglong.odata.cc || true
    
    # 3. 安装玲珑商店
    info "安装玲珑商店..."
    sudo ll-cli install com.dongpl.linglong-store.v2 --repo testing
    
    echo ""
    info "安装完成！"
}

# 执行主函数
main "$@"
