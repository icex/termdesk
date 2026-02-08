#!/usr/bin/env bash
set -euo pipefail

# ---------------------------------------------------------------------------
#  termdesk install script -- Linux, macOS, and Android (Termux)
# ---------------------------------------------------------------------------

# -- colours ----------------------------------------------------------------
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
BOLD='\033[1m'
RESET='\033[0m'

info()    { printf "${BLUE}[info]${RESET}  %s\n" "$*"; }
ok()      { printf "${GREEN}[ok]${RESET}    %s\n" "$*"; }
warn()    { printf "${YELLOW}[warn]${RESET}  %s\n" "$*"; }
err()     { printf "${RED}[error]${RESET} %s\n" "$*"; }
heading() { printf "\n${BOLD}${CYAN}%s${RESET}\n" "$*"; }

# -- globals ----------------------------------------------------------------
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MIN_GO_MAJOR=1
MIN_GO_MINOR=23
GO_CMD=""
OS=""       # linux, macos, termux
PKG_MGR=""  # apt, brew, pkg, dnf, pacman

# -- detect OS and package manager -----------------------------------------
detect_os() {
    heading "Detecting operating system"

    # Check for Termux first (Android)
    if [[ -n "${TERMUX_VERSION:-}" ]] || [[ -d "/data/data/com.termux" ]]; then
        OS="termux"
        PKG_MGR="pkg"
        ok "Detected Termux on Android ($(uname -m))"
        return
    fi

    case "$(uname -s)" in
        Linux*)  OS="linux";;
        Darwin*) OS="macos";;
        *)       err "Unsupported OS: $(uname -s)"; exit 1;;
    esac

    # Detect package manager
    if [[ "${OS}" == "macos" ]]; then
        if command -v brew &>/dev/null; then
            PKG_MGR="brew"
        fi
    elif [[ "${OS}" == "linux" ]]; then
        if command -v apt &>/dev/null; then
            PKG_MGR="apt"
        elif command -v dnf &>/dev/null; then
            PKG_MGR="dnf"
        elif command -v pacman &>/dev/null; then
            PKG_MGR="pacman"
        fi
    fi

    ok "Detected ${OS} ($(uname -m))${PKG_MGR:+, package manager: ${PKG_MGR}}"
}

# -- locate Go and verify version -------------------------------------------
check_go() {
    heading "Checking Go installation"

    # Try common locations
    if command -v go &>/dev/null; then
        GO_CMD="$(command -v go)"
    elif [[ -x "${HOME}/.local/go/bin/go" ]]; then
        GO_CMD="${HOME}/.local/go/bin/go"
    elif [[ -x "/usr/local/go/bin/go" ]]; then
        GO_CMD="/usr/local/go/bin/go"
    elif [[ -x "${HOME}/go/bin/go" ]]; then
        GO_CMD="${HOME}/go/bin/go"
    fi

    if [[ -z "${GO_CMD}" ]]; then
        err "Go is not installed."
        echo ""
        echo "  Install Go ${MIN_GO_MAJOR}.${MIN_GO_MINOR}+ :"
        echo ""
        case "${OS}" in
            termux)
                echo "    pkg install golang";;
            macos)
                echo "    brew install go"
                echo "    -- or download from https://go.dev/dl/";;
            linux)
                echo "    Download from https://go.dev/dl/"
                echo ""
                echo "    Quick install:"
                echo "      wget https://go.dev/dl/go1.23.6.linux-amd64.tar.gz"
                echo "      sudo tar -C /usr/local -xzf go1.23.6.linux-amd64.tar.gz"
                echo "      export PATH=/usr/local/go/bin:\$PATH";;
        esac
        echo ""
        exit 1
    fi

    # Parse version
    local go_version
    go_version="$("${GO_CMD}" version | grep -oE 'go[0-9]+\.[0-9]+(\.[0-9]+)?' | head -1)"
    local major minor
    major="$(echo "${go_version}" | sed -E 's/go([0-9]+)\..*/\1/')"
    minor="$(echo "${go_version}" | sed -E 's/go[0-9]+\.([0-9]+).*/\1/')"

    if [[ "${major}" -lt "${MIN_GO_MAJOR}" ]] || \
       { [[ "${major}" -eq "${MIN_GO_MAJOR}" ]] && [[ "${minor}" -lt "${MIN_GO_MINOR}" ]]; }; then
        err "Go ${go_version} found, but ${MIN_GO_MAJOR}.${MIN_GO_MINOR}+ is required."
        echo "  Upgrade at: https://go.dev/dl/"
        exit 1
    fi

    ok "Found ${go_version} at ${GO_CMD}"
}

# -- check for Nerd Font -----------------------------------------------------
check_nerd_font() {
    heading "Checking for Nerd Font"

    local found=false

    if [[ "${OS}" == "termux" ]]; then
        # Termux uses ~/.termux/font.ttf
        if [[ -f "${HOME}/.termux/font.ttf" ]]; then
            found=true
        fi
    elif command -v fc-list &>/dev/null; then
        if fc-list 2>/dev/null | grep -qi "nerd\|nf-"; then
            found=true
        fi
    fi

    # On macOS also check the font directories directly
    if [[ "${OS}" == "macos" ]] && [[ "${found}" == false ]]; then
        for dir in ~/Library/Fonts /Library/Fonts /System/Library/Fonts; do
            if ls "${dir}" 2>/dev/null | grep -qi "nerd\|NF"; then
                found=true
                break
            fi
        done
    fi

    if [[ "${found}" == true ]]; then
        ok "Nerd Font detected"
    else
        warn "No Nerd Font detected. termdesk uses icons that require a Nerd Font."
        echo ""
        echo "  Recommended: JetBrainsMono Nerd Font"
        echo "  Download:    https://www.nerdfonts.com/font-downloads"
        echo ""
        case "${OS}" in
            termux)
                echo "  Quick install (Termux):"
                echo "    curl -fLo ~/.termux/font.ttf \\"
                echo "      https://github.com/ryanoasis/nerd-fonts/raw/HEAD/patched-fonts/JetBrainsMono/Ligatures/Regular/JetBrainsMonoNerdFont-Regular.ttf"
                echo "    termux-reload-settings";;
            macos)
                echo "  Quick install (Homebrew):"
                echo "    brew install --cask font-jetbrains-mono-nerd-font";;
            linux)
                echo "  Quick install (Linux):"
                echo "    mkdir -p ~/.local/share/fonts"
                echo "    cd ~/.local/share/fonts"
                echo "    curl -fLO https://github.com/ryanoasis/nerd-fonts/releases/latest/download/JetBrainsMono.tar.xz"
                echo "    tar -xf JetBrainsMono.tar.xz"
                echo "    fc-cache -fv";;
        esac
        echo ""
        info "Continuing with install..."
    fi
}

# -- install recommended apps -----------------------------------------------
install_apps() {
    heading "Checking recommended apps"

    # Apps used by termdesk dock/launcher/menubar
    local apps=("btop" "nvim" "python3")
    local missing=()

    for app in "${apps[@]}"; do
        local cmd="${app}"
        # nvim binary is nvim
        if command -v "${cmd}" &>/dev/null; then
            ok "${app} found"
        else
            missing+=("${app}")
            warn "${app} not found"
        fi
    done

    if [[ ${#missing[@]} -eq 0 ]]; then
        ok "All recommended apps are installed"
        return
    fi

    echo ""
    info "Missing apps: ${missing[*]}"
    echo ""

    # Map app names to package names per platform
    local pkgs=()
    for app in "${missing[@]}"; do
        case "${OS}:${app}" in
            termux:nvim)    pkgs+=("neovim");;
            termux:btop)    pkgs+=("btop");;
            termux:python3) pkgs+=("python");;
            macos:nvim)     pkgs+=("neovim");;
            macos:btop)     pkgs+=("btop");;
            macos:python3)  pkgs+=("python3");;
            linux:nvim)     pkgs+=("neovim");;
            linux:btop)     pkgs+=("btop");;
            linux:python3)  pkgs+=("python3");;
        esac
    done

    if [[ ${#pkgs[@]} -eq 0 ]]; then
        return
    fi

    if [[ -z "${PKG_MGR}" ]]; then
        warn "No package manager detected. Install manually: ${pkgs[*]}"
        return
    fi

    echo "  Install missing apps with:"
    echo ""
    case "${PKG_MGR}" in
        pkg)    echo "    pkg install ${pkgs[*]}";;
        brew)   echo "    brew install ${pkgs[*]}";;
        apt)    echo "    sudo apt install ${pkgs[*]}";;
        dnf)    echo "    sudo dnf install ${pkgs[*]}";;
        pacman) echo "    sudo pacman -S ${pkgs[*]}";;
    esac
    echo ""

    # Auto-install on Termux (no sudo needed)
    if [[ "${PKG_MGR}" == "pkg" ]]; then
        read -rp "  Install now? [Y/n] " answer
        answer="${answer:-y}"
        if [[ "${answer}" =~ ^[Yy] ]]; then
            pkg install -y "${pkgs[@]}"
            ok "Apps installed"
        else
            info "Skipping app install"
        fi
    else
        info "Install them after termdesk is set up (optional but recommended)"
    fi
}

# -- build -------------------------------------------------------------------
build() {
    heading "Building termdesk"

    cd "${SCRIPT_DIR}"
    mkdir -p bin

    # Ensure GOPATH/bin is in PATH for go install dependencies
    export PATH="${HOME}/go/bin:${HOME}/.local/go/bin:/usr/local/go/bin:${PATH}"

    local build_flags=()
    # Android/Termux requires PIE (Position Independent Executable)
    if [[ "${OS}" == "termux" ]]; then
        build_flags+=("-buildmode=pie")
    fi

    info "Running: ${GO_CMD} build ${build_flags[*]} -o bin/termdesk ./cmd/termdesk"
    "${GO_CMD}" build "${build_flags[@]}" -o bin/termdesk ./cmd/termdesk

    if [[ ! -x bin/termdesk ]]; then
        err "Build failed -- binary not found at bin/termdesk"
        exit 1
    fi

    ok "Built bin/termdesk successfully"
}

# -- install binary ----------------------------------------------------------
install_binary() {
    heading "Installing binary"

    local install_dir

    case "${OS}" in
        termux)
            install_dir="${PREFIX}/bin";;
        macos)
            if [[ -w /usr/local/bin ]]; then
                install_dir="/usr/local/bin"
            else
                install_dir="${HOME}/.local/bin"
            fi;;
        linux)
            install_dir="${HOME}/.local/bin";;
    esac

    mkdir -p "${install_dir}"
    cp "${SCRIPT_DIR}/bin/termdesk" "${install_dir}/termdesk"
    chmod +x "${install_dir}/termdesk"

    ok "Installed to ${install_dir}/termdesk"

    # Warn if the directory is not in PATH
    if ! echo "${PATH}" | tr ':' '\n' | grep -qx "${install_dir}"; then
        warn "${install_dir} is not in your PATH."
        echo ""
        echo "  Add it by appending this to your shell profile (~/.bashrc, ~/.zshrc, etc.):"
        echo "    export PATH=\"${install_dir}:\${PATH}\""
    fi
}

# -- create .desktop file (Linux desktop only) ------------------------------
create_desktop_entry() {
    # Skip on Termux and macOS
    if [[ "${OS}" != "linux" ]]; then
        return
    fi

    heading "Creating .desktop entry"

    local desktop_dir="${HOME}/.local/share/applications"
    mkdir -p "${desktop_dir}"

    local desktop_file="${desktop_dir}/termdesk.desktop"
    local bin_path="${HOME}/.local/bin/termdesk"

    cat > "${desktop_file}" <<EOF
[Desktop Entry]
Name=Termdesk
Comment=A TUI desktop environment
Exec=${bin_path}
Terminal=true
Type=Application
Categories=Development;System;TerminalEmulator;
Keywords=terminal;tui;desktop;tiling;
Icon=utilities-terminal
StartupNotify=false
EOF

    ok "Created ${desktop_file}"

    # Update desktop database if available
    if command -v update-desktop-database &>/dev/null; then
        update-desktop-database "${desktop_dir}" 2>/dev/null || true
    fi
}

# -- done --------------------------------------------------------------------
print_success() {
    heading "Installation complete!"

    echo ""
    printf "  ${GREEN}${BOLD}termdesk${RESET} has been installed successfully.\n"
    echo ""
    echo "  Usage:"
    echo "    termdesk          Launch termdesk"
    echo ""

    case "${OS}" in
        linux)
            echo "  A .desktop entry was created so termdesk appears in your"
            echo "  application launcher."
            echo "";;
        termux)
            echo "  Tip: For best experience in Termux, set your font to a"
            echo "  Nerd Font and use a larger font size (Settings > Style)."
            echo "";;
    esac

    echo "  Make sure your terminal is using a Nerd Font for the best experience."
    echo ""
}

# -- main --------------------------------------------------------------------
main() {
    printf "${BOLD}${CYAN}"
    echo "  _                          _           _    "
    echo " | |_ ___ _ __ _ __ ___   __| | ___  ___| | __"
    echo " | __/ _ \ '__| '_ \` _ \ / _\` |/ _ \/ __| |/ /"
    echo " | ||  __/ |  | | | | | | (_| |  __/\__ \   < "
    echo "  \__\___|_|  |_| |_| |_|\__,_|\___||___/_|\_\\"
    printf "${RESET}\n"
    echo "  installer"
    echo ""

    detect_os
    check_go
    check_nerd_font
    install_apps
    build
    install_binary
    create_desktop_entry
    print_success
}

main "$@"
