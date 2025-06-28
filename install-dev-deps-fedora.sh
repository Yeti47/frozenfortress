#!/bin/bash

# FrozenFortress Development Dependencies Installer for Fedora
# This script installs all necessary development dependencies for FrozenFortress

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Logging functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if running as root
check_root() {
    if [[ $EUID -eq 0 ]]; then
        log_error "This script should not be run as root for security reasons."
        log_info "Please run as a regular user. The script will prompt for sudo when needed."
        exit 1
    fi
}

# Check if dnf is available
check_package_manager() {
    if ! command -v dnf &> /dev/null; then
        log_error "dnf package manager not found. This script is designed for Fedora systems."
        exit 1
    fi
}

# Function to compare version numbers
version_compare() {
    local version1=$1
    local version2=$2
    
    # Convert versions to comparable format
    local ver1=$(echo "$version1" | sed 's/[^0-9.]*//g')
    local ver2=$(echo "$version2" | sed 's/[^0-9.]*//g')
    
    # Use sort -V for version comparison
    local result=$(printf '%s\n%s' "$ver1" "$ver2" | sort -V | head -n1)
    
    if [[ "$result" == "$ver2" ]]; then
        return 0  # version1 >= version2
    else
        return 1  # version1 < version2
    fi
}

# Check and install Go
install_go() {
    local required_version="1.24.3"
    local go_install_dir="/usr/local"
    local go_tar_file="go1.24.3.linux-amd64.tar.gz"
    local go_download_url="https://go.dev/dl/go1.24.3.linux-amd64.tar.gz"
    
    log_info "Checking Go installation..."
    
    if command -v go &> /dev/null; then
        local current_version=$(go version | grep -o 'go[0-9.]*' | sed 's/go//')
        log_info "Found Go version: $current_version"
        
        if version_compare "$current_version" "$required_version"; then
            log_success "Go version $current_version meets requirement (>= $required_version)"
            return 0
        else
            log_warning "Go version $current_version is below required version $required_version"
        fi
    else
        log_info "Go not found in PATH"
    fi
    
    # Check if there's an existing manual installation that would conflict
    if [[ -d "$go_install_dir/go" ]]; then
        log_warning "Found existing Go installation at $go_install_dir/go"
        echo "To install Go $required_version, the existing installation needs to be removed."
        echo -n "Do you want to remove the existing Go installation and install Go $required_version? (y/N): "
        read -r response
        
        if [[ ! "$response" =~ ^[Yy]$ ]]; then
            log_info "Skipping Go installation. Existing installation preserved."
            log_warning "Note: The existing Go version may not meet the project requirements."
            return 0
        fi
        
        log_info "Removing existing Go installation with user consent..."
        sudo rm -rf "$go_install_dir/go"
    fi
    
    log_info "Installing Go $required_version..."
    
    # Create temporary directory
    local temp_dir=$(mktemp -d)
    cd "$temp_dir"
    
    # Download Go
    log_info "Downloading Go from $go_download_url..."
    if ! wget "$go_download_url"; then
        log_error "Failed to download Go. Please check your internet connection."
        rm -rf "$temp_dir"
        exit 1
    fi
    
    # Verify download
    if [[ ! -f "$go_tar_file" ]]; then
        log_error "Go download failed - file not found"
        rm -rf "$temp_dir"
        exit 1
    fi
    
    # Extract Go
    log_info "Installing Go to $go_install_dir..."
    sudo tar -C "$go_install_dir" -xzf "$go_tar_file"
    
    # Cleanup
    cd - > /dev/null
    rm -rf "$temp_dir"
    
    # Set up environment variables
    local go_path="$go_install_dir/go/bin"
    
    # Add to current session
    export PATH="$go_path:$PATH"
    
    # Check if Go is in PATH for current session
    if command -v go &> /dev/null; then
        local installed_version=$(go version | grep -o 'go[0-9.]*' | sed 's/go//')
        log_success "Go $installed_version installed successfully"
    else
        log_error "Go installation failed - not found in PATH after installation"
        exit 1
    fi
    
    # Add to shell profile for persistence
    local shell_profile=""
    if [[ -f "$HOME/.bashrc" ]]; then
        shell_profile="$HOME/.bashrc"
    elif [[ -f "$HOME/.zshrc" ]]; then
        shell_profile="$HOME/.zshrc"
    elif [[ -f "$HOME/.profile" ]]; then
        shell_profile="$HOME/.profile"
    fi
    
    if [[ -n "$shell_profile" ]]; then
        if ! grep -q "$go_path" "$shell_profile"; then
            echo "" >> "$shell_profile"
            echo "# Go installation" >> "$shell_profile"
            echo "export PATH=\"$go_path:\$PATH\"" >> "$shell_profile"
            log_success "Added Go to PATH in $shell_profile"
            log_info "Please run 'source $shell_profile' or restart your terminal to make the PATH change permanent"
        else
            log_info "Go PATH already configured in $shell_profile"
        fi
    else
        log_warning "Could not find shell profile to update PATH automatically"
        log_info "Please add the following line to your shell profile:"
        log_info "export PATH=\"$go_path:\$PATH\""
    fi
}

# Install Redis (Valkey)
install_redis() {
    log_info "Installing Redis (Valkey package)..."
    
    if command -v redis-server &> /dev/null || command -v valkey-server &> /dev/null; then
        log_success "Redis/Valkey is already installed"
        return 0
    fi
    
    sudo dnf install -y valkey
    
    # Enable and start the service
    sudo systemctl enable valkey
    sudo systemctl start valkey
    
    log_success "Redis (Valkey) installed and started successfully"
}

# Install Tesseract and dependencies
install_tesseract() {
    log_info "Installing Tesseract OCR and dependencies..."
    
    # Install base packages
    local packages=(
        "tesseract"
        "tesseract-devel"
        "leptonica"
        "leptonica-devel"
    )
    
    for package in "${packages[@]}"; do
        log_info "Installing $package..."
        sudo dnf install -y "$package"
    done
    
    log_success "Tesseract base packages installed successfully"
}

# Install Tesseract language packs
install_tesseract_languages() {
    log_info "Setting up Tesseract language packs..."
    
    # Available common languages
    local available_languages=(
        "eng:English"
        "deu:German"
        "spa:Spanish"
        "fra:French"
        "ita:Italian"
        "por:Portuguese"
        "rus:Russian"
        "chi_sim:Chinese Simplified"
        "chi_tra:Chinese Traditional"
        "jpn:Japanese"
        "kor:Korean"
        "ara:Arabic"
        "hin:Hindi"
        "nld:Dutch"
        "swe:Swedish"
        "nor:Norwegian"
        "dan:Danish"
        "fin:Finnish"
    )
    
    echo ""
    log_info "Available Tesseract language packs:"
    echo "Default: eng (English) - will be installed automatically"
    echo ""
    
    for i in "${!available_languages[@]}"; do
        IFS=':' read -r code name <<< "${available_languages[$i]}"
        printf "%2d) %s (%s)\n" $((i+1)) "$name" "$code"
    done
    
    echo ""
    echo "Enter the numbers of additional languages to install (space-separated),"
    echo "or press Enter to install only English:"
    read -r language_selection
    
    # Always install English
    local selected_languages=("eng")
    
    # Process user selection
    if [[ -n "$language_selection" ]]; then
        for selection in $language_selection; do
            if [[ "$selection" =~ ^[0-9]+$ ]] && [[ "$selection" -ge 1 ]] && [[ "$selection" -le "${#available_languages[@]}" ]]; then
                local index=$((selection-1))
                IFS=':' read -r code name <<< "${available_languages[$index]}"
                if [[ "$code" != "eng" ]]; then  # Don't duplicate English
                    selected_languages+=("$code")
                fi
            else
                log_warning "Invalid selection: $selection (ignored)"
            fi
        done
    fi
    
    # Install selected language packs
    for lang_code in "${selected_languages[@]}"; do
        local package_name="tesseract-langpack-$lang_code"
        log_info "Installing $package_name..."
        
        if sudo dnf install -y "$package_name"; then
            log_success "Installed language pack: $lang_code"
        else
            log_warning "Failed to install language pack: $lang_code (may not be available)"
        fi
    done
    
    log_success "Tesseract language pack installation completed"
}

# Verify installations
verify_installations() {
    log_info "Verifying installations..."
    
    # Check Go
    if command -v go &> /dev/null; then
        local go_version=$(go version | grep -o 'go[0-9.]*' | sed 's/go//')
        log_success "Go $go_version is installed and accessible"
    else
        log_error "Go verification failed"
    fi
    
    # Check Redis/Valkey
    if command -v redis-server &> /dev/null || command -v valkey-server &> /dev/null; then
        log_success "Redis/Valkey is installed"
        if systemctl is-active --quiet valkey; then
            log_success "Valkey service is running"
        else
            log_warning "Valkey service is not running"
        fi
    else
        log_error "Redis/Valkey verification failed"
    fi
    
    # Check Tesseract
    if command -v tesseract &> /dev/null; then
        local tesseract_version=$(tesseract --version 2>&1 | head -n1)
        log_success "Tesseract is installed: $tesseract_version"
        
        # List available languages
        local languages=$(tesseract --list-langs 2>/dev/null | tail -n +2 | tr '\n' ' ')
        log_info "Available Tesseract languages: $languages"
    else
        log_error "Tesseract verification failed"
    fi
}

# Main installation function
main() {
    echo "======================================================"
    echo "FrozenFortress Development Dependencies Installer"
    echo "======================================================"
    echo ""
    
    # Preliminary checks
    check_root
    check_package_manager
    
    # Update package database
    log_info "Updating package database..."
    sudo dnf update -y
    
    # Install dependencies
    install_go
    install_redis
    install_tesseract
    install_tesseract_languages
    
    # Verify everything is working
    verify_installations
    
    echo ""
    echo "======================================================"
    log_success "FrozenFortress development dependencies installation completed!"
    echo "======================================================"
    echo ""
    log_info "Next steps:"
    echo "1. If Go PATH was updated, restart your terminal or run: source ~/.bashrc"
    echo "2. Test the installation by running: go version"
    echo "3. Build the project with: ./build-all.sh"
    echo ""
}

# Run main function
main "$@"
