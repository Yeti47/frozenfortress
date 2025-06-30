#!/bin/bash

# Frozen Fortress Setup Script
# This script helps set up Frozen Fortress on a Linux system
# It handles dependencies, installation, and optional nginx configuration

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Default values
DEFAULT_INSTALL_DIR="$HOME/frozenfortress"
DEFAULT_NGINX_PORT=8443
DEFAULT_CERT_DAYS=3650  # 10 years

# Function to print colored output
print_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

# Function to check if running as root
check_root() {
    if [[ $EUID -eq 0 ]]; then
        print_error "This script should not be run as root for security reasons."
        print_info "Please run as a regular user. The script will prompt for sudo when needed."
        exit 1
    fi
}

# Function to detect package manager
detect_package_manager() {
    if command -v apt-get &> /dev/null; then
        echo "apt"
    elif command -v yum &> /dev/null; then
        echo "yum"
    elif command -v dnf &> /dev/null; then
        echo "dnf"
    elif command -v pacman &> /dev/null; then
        echo "pacman"
    else
        echo "unknown"
    fi
}

# Function to install packages based on package manager
install_packages() {
    local pkg_manager=$1
    shift
    local packages=("$@")
    
    case $pkg_manager in
        apt)
            print_info "Installing packages with apt: ${packages[*]}"
            sudo apt-get update
            sudo apt-get install -y "${packages[@]}"
            ;;
        yum)
            print_info "Installing packages with yum: ${packages[*]}"
            sudo yum install -y "${packages[@]}"
            ;;
        dnf)
            print_info "Installing packages with dnf: ${packages[*]}"
            sudo dnf install -y "${packages[@]}"
            ;;
        pacman)
            print_info "Installing packages with pacman: ${packages[*]}"
            sudo pacman -S --noconfirm "${packages[@]}"
            ;;
        *)
            print_error "Unsupported package manager. Please install the following packages manually: ${packages[*]}"
            exit 1
            ;;
    esac
}

# Function to install dependencies
install_dependencies() {
    print_info "Installing Frozen Fortress dependencies..."
    
    local pkg_manager=$(detect_package_manager)
    if [[ "$pkg_manager" == "unknown" ]]; then
        print_error "Could not detect package manager. Please install dependencies manually:"
        print_info "Required: tesseract-ocr, libleptonica-dev, redis-server, tesseract-ocr-eng"
        exit 1
    fi
    
    # Define packages for different package managers
    case $pkg_manager in
        apt)
            local packages=(tesseract-ocr libleptonica-dev redis-server tesseract-ocr-eng unzip)
            ;;
        yum|dnf)
            local packages=(tesseract leptonica-devel redis tesseract-langpack-eng unzip)
            ;;
        pacman)
            local packages=(tesseract leptonica redis tesseract-data-eng unzip)
            ;;
    esac
    
    install_packages "$pkg_manager" "${packages[@]}"
    
    # Start and enable redis
    print_info "Starting Redis service..."
    sudo systemctl start redis
    sudo systemctl enable redis
    
    print_success "Dependencies installed successfully!"
}

# Function to prompt for additional Tesseract language packs
install_tesseract_languages() {
    print_info "Available Tesseract language packs (common ones):"
    echo "  deu - German"
    echo "  fra - French"
    echo "  spa - Spanish"
    echo "  ita - Italian"
    echo "  por - Portuguese"
    echo "  rus - Russian"
    echo "  chi_sim - Chinese Simplified"
    echo "  jpn - Japanese"
    echo "  ara - Arabic"
    
    echo ""
    read -p "Enter additional language codes (space-separated, or press Enter to skip): " -r additional_langs
    
    if [[ -n "$additional_langs" ]]; then
        local pkg_manager=$(detect_package_manager)
        for lang in $additional_langs; do
            case $pkg_manager in
                apt)
                    local pkg="tesseract-ocr-$lang"
                    ;;
                yum|dnf)
                    local pkg="tesseract-langpack-$lang"
                    ;;
                pacman)
                    local pkg="tesseract-data-$lang"
                    ;;
            esac
            
            print_info "Installing language pack: $lang"
            install_packages "$pkg_manager" "$pkg" || print_warning "Failed to install language pack: $lang"
        done
    fi
}

# Function to install Frozen Fortress
install_frozenfortress() {
    echo ""
    print_info "Frozen Fortress Installation"
    echo "=============================="
    
    read -p "Installation directory (default: $DEFAULT_INSTALL_DIR): " -r install_dir
    install_dir=${install_dir:-$DEFAULT_INSTALL_DIR}
    
    # Expand tilde
    install_dir=$(eval echo "$install_dir")
    
    print_info "Installing to: $install_dir"
    mkdir -p "$install_dir"
    
    # Find the zip file in the current directory
    zip_file=$(find . -name "frozenfortress-*.zip" -type f | head -n 1)
    if [[ -z "$zip_file" ]]; then
        print_error "No Frozen Fortress zip file found in current directory!"
        exit 1
    fi
    
    print_info "Extracting $zip_file to $install_dir"
    unzip -o "$zip_file" -d "$install_dir"
    
    # Make binaries executable
    chmod +x "$install_dir"/{ffcli,ffwebui,ffcli-debug,ffwebui-debug} 2>/dev/null || true
    
    # Create symlinks in user's local bin if it exists
    if [[ -d "$HOME/.local/bin" ]]; then
        print_info "Creating symlinks in ~/.local/bin"
        ln -sf "$install_dir/ffcli" "$HOME/.local/bin/ffcli" 2>/dev/null || true
        ln -sf "$install_dir/ffwebui" "$HOME/.local/bin/ff-webui" 2>/dev/null || true
    fi
    
    print_success "Frozen Fortress installed successfully!"
    echo "Installation path: $install_dir"
    
    # Store install path for later use
    echo "$install_dir" > /tmp/ff_install_path
}

# Function to generate self-signed certificate
generate_ssl_cert() {
    local cert_dir=$1
    local days=$2
    local domain=${3:-localhost}
    
    print_info "Generating self-signed SSL certificate..."
    
    sudo openssl req -x509 -nodes -days "$days" -newkey rsa:2048 \
        -keyout "$cert_dir/frozenfortress.key" \
        -out "$cert_dir/frozenfortress.crt" \
        -subj "/C=US/ST=Local/L=Local/O=FrozenFortress/CN=$domain" \
        -addext "subjectAltName=DNS:localhost,DNS:127.0.0.1,IP:127.0.0.1"
    
    sudo chmod 600 "$cert_dir/frozenfortress.key"
    sudo chmod 644 "$cert_dir/frozenfortress.crt"
    
    print_success "SSL certificate generated successfully!"
}

# Function to setup nginx
setup_nginx() {
    echo ""
    read -p "Do you want to set up nginx as a reverse proxy? (y/N): " -r setup_nginx
    
    if [[ ! "$setup_nginx" =~ ^[Yy]$ ]]; then
        return 0
    fi
    
    local pkg_manager=$(detect_package_manager)
    
    # Check if nginx is installed
    if ! command -v nginx &> /dev/null; then
        print_info "Installing nginx..."
        case $pkg_manager in
            apt)
                install_packages "$pkg_manager" nginx
                ;;
            yum|dnf)
                install_packages "$pkg_manager" nginx
                ;;
            pacman)
                install_packages "$pkg_manager" nginx
                ;;
        esac
    else
        print_info "nginx is already installed"
    fi
    
    # Check if openssl is installed
    if ! command -v openssl &> /dev/null; then
        print_info "Installing OpenSSL..."
        case $pkg_manager in
            apt)
                install_packages "$pkg_manager" openssl
                ;;
            yum|dnf)
                install_packages "$pkg_manager" openssl
                ;;
            pacman)
                install_packages "$pkg_manager" openssl
                ;;
        esac
    else
        print_info "OpenSSL is already installed"
    fi
    
    # Get configuration from user
    read -p "HTTPS port for nginx (default: $DEFAULT_NGINX_PORT): " -r nginx_port
    nginx_port=${nginx_port:-$DEFAULT_NGINX_PORT}
    
    read -p "Certificate validity in days (default: $DEFAULT_CERT_DAYS): " -r cert_days
    cert_days=${cert_days:-$DEFAULT_CERT_DAYS}
    
    read -p "Domain/hostname for certificate (default: localhost): " -r cert_domain
    cert_domain=${cert_domain:-localhost}
    
    # Create SSL directory
    local ssl_dir="/etc/ssl/frozenfortress"
    sudo mkdir -p "$ssl_dir"
    
    # Generate certificate
    generate_ssl_cert "$ssl_dir" "$cert_days" "$cert_domain"
    
    # Create nginx configuration
    local nginx_config="/etc/nginx/sites-available/frozenfortress"
    local install_dir=$(cat /tmp/ff_install_path)
    
    # Create sites-available and sites-enabled directories if they don't exist
    sudo mkdir -p /etc/nginx/sites-available
    sudo mkdir -p /etc/nginx/sites-enabled
    
    print_info "Creating nginx configuration..."
    sudo tee "$nginx_config" > /dev/null <<EOF
server {
    listen $nginx_port ssl http2;
    server_name $cert_domain localhost;
    
    ssl_certificate $ssl_dir/frozenfortress.crt;
    ssl_certificate_key $ssl_dir/frozenfortress.key;
    
    # SSL Configuration
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers ECDHE-RSA-AES128-GCM-SHA256:ECDHE-RSA-AES256-GCM-SHA384:ECDHE-RSA-AES128-SHA256:ECDHE-RSA-AES256-SHA384;
    ssl_prefer_server_ciphers on;
    ssl_session_cache shared:SSL:10m;
    ssl_session_timeout 10m;
    
    # Security headers
    add_header X-Frame-Options DENY;
    add_header X-Content-Type-Options nosniff;
    add_header X-XSS-Protection "1; mode=block";
    add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;
    
    # Proxy configuration
    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
        proxy_set_header X-Forwarded-Host \$host;
        proxy_set_header X-Forwarded-Port \$server_port;
        
        # WebSocket support
        proxy_http_version 1.1;
        proxy_set_header Upgrade \$http_upgrade;
        proxy_set_header Connection "upgrade";
        
        # Timeouts
        proxy_connect_timeout 60s;
        proxy_send_timeout 60s;
        proxy_read_timeout 60s;
    }
    
    # Static files (if served directly)
    location /static {
        alias $install_dir/static;
        expires 1y;
        add_header Cache-Control "public, immutable";
    }
    
    location /img {
        alias $install_dir/img;
        expires 1y;
        add_header Cache-Control "public, immutable";
    }
}
EOF
    
    # Enable the site
    sudo ln -sf "$nginx_config" /etc/nginx/sites-enabled/frozenfortress
    
    # Test nginx configuration
    print_info "Testing nginx configuration..."
    sudo nginx -t
    
    # Start and enable nginx
    print_info "Starting nginx service..."
    sudo systemctl start nginx
    sudo systemctl enable nginx
    sudo systemctl reload nginx
    
    print_success "nginx configured successfully!"
    print_info "Frozen Fortress will be available at: https://$cert_domain:$nginx_port"
    print_warning "Note: You'll need to accept the self-signed certificate in your browser"
}

# Function to show final instructions
show_final_instructions() {
    echo ""
    echo "=============================="
    print_success "Setup Complete!"
    echo "=============================="
    
    local install_dir=$(cat /tmp/ff_install_path)
    
    echo ""
    print_info "Installation Summary:"
    echo "  Installation directory: $install_dir"
    echo "  CLI executable: $install_dir/ffcli"
    echo "  WebUI executable: $install_dir/ffwebui"
    
    if [[ -f "/etc/nginx/sites-enabled/frozenfortress" ]]; then
        local nginx_port=$(grep "listen.*ssl" /etc/nginx/sites-enabled/frozenfortress | grep -o '[0-9]\+' | head -n1)
        local cert_domain=$(grep "server_name" /etc/nginx/sites-enabled/frozenfortress | awk '{print $2}' | sed 's/;//')
        echo "  Web interface: https://$cert_domain:$nginx_port"
    fi
    
    echo ""
    print_info "Next Steps:"
    echo "1. Configure Frozen Fortress:"
    echo "   cd $install_dir"
    echo "   ./ffcli setup"
    echo ""
    echo "2. Start the WebUI:"
    echo "   cd $install_dir"
    echo "   ./ffwebui"
    echo ""
    echo "3. Important locations:"
    echo "   - Database: ~/.config/frozenfortress/"
    echo "   - Logs: ~/.config/frozenfortress/logs/"
    echo "   - Backups: ~/.config/frozenfortress/backups/"
    echo "   - Session keys: ~/.config/frozenfortress/ (or custom FF_KEY_DIR)"
    echo ""
    print_info "For more information, run:"
    echo "   ./ffcli --help"
    echo ""
    print_warning "Remember to backup your database regularly!"
    print_info "Note: Session keys are auto-generated and don't need backup (only used for web sessions)."
    
    # Cleanup
    rm -f /tmp/ff_install_path
}

# Main function
main() {
    echo "=============================="
    echo "Frozen Fortress Setup Script"
    echo "=============================="
    echo ""
    
    check_root
    
    print_info "This script will help you set up Frozen Fortress on your Linux system."
    print_info "It will install dependencies, set up the application, and optionally configure nginx."
    echo ""
    
    read -p "Continue with setup? (Y/n): " -r continue_setup
    if [[ "$continue_setup" =~ ^[Nn]$ ]]; then
        print_info "Setup cancelled."
        exit 0
    fi
    
    # Step 1: Install dependencies
    print_info "Step 1: Installing dependencies..."
    install_dependencies
    
    # Step 2: Install additional Tesseract languages
    print_info "Step 2: Installing additional Tesseract language packs..."
    install_tesseract_languages
    
    # Step 3: Install Frozen Fortress
    print_info "Step 3: Installing Frozen Fortress..."
    install_frozenfortress
    
    # Step 4: Setup nginx (optional)
    print_info "Step 4: Setting up nginx (optional)..."
    setup_nginx
    
    # Step 5: Show final instructions
    show_final_instructions
}

# Run main function
main "$@"
