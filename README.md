# FrozenFortress

FrozenFortress is a **lightweight secret and document manager** designed for local self-hosting. Built with **Go**, it provides a secure, simple, and pragmatic solution for storing and managing sensitive information like passwords, secrets, and documents in a local environment.

## 🎯 What is FrozenFortress?

FrozenFortress is designed to help individuals and small teams manage their sensitive data locally without relying on cloud services. It provides:

- **Secret Management**: Store and organize passwords, API keys, and other sensitive information
- **Document Management**: Store and organize documents with OCR support for text extraction
- **User Management**: Multi-user support with authentication and authorization
- **Web Interface**: Modern web UI for easy interaction
- **CLI Tools**: Command-line interface for administrative tasks
- **Backup System**: Automated backup functionality to protect your data
- **OCR Support**: Text extraction from images and PDFs using Tesseract

## 🏗️ Architecture & Tech Stack

FrozenFortress is built with **simplicity and pragmatism** as core driving principles. The architecture prioritizes:

- **Minimal Dependencies**: Uses SQLite for data storage, eliminating the need for complex database setups
- **Self-Contained**: All components can run on a single machine
- **Modular Design**: Clean separation between core business logic, web UI, and CLI components
- **Security First**: Built-in encryption, secure session management, and proper authentication

### Technology Stack

- **Backend**: Go 1.24.3
- **Database**: SQLite 3
- **Session Storage**: Redis (required for session management)
- **Web Framework**: Gin (HTTP web framework)
- **CLI Framework**: Cobra (command-line interface)
- **OCR**: Tesseract (optical character recognition)
- **Authentication**: Session-based with secure cookies
- **Encryption**: Built-in encryption services for data protection

### Project Structure

```
frozenfortress/
├── cli/                    # Command-line interface
│   ├── main.go            # CLI entry point
│   └── cmd/               # CLI commands (user management, setup, backup)
├── webui/                 # Web user interface
│   ├── main.go            # Web server entry point
│   ├── views/             # HTML templates
│   └── middleware/        # HTTP middleware
├── core/                  # Core business logic
│   ├── auth/              # Authentication & user management
│   ├── secrets/           # Secret management
│   ├── documents/         # Document management
│   ├── encryption/        # Encryption services
│   ├── backup/            # Backup functionality
│   └── ccc/               # Common core components
└── bin/                   # Compiled binaries
```

## 🚀 Getting Started

### Prerequisites

- **Go 1.24.3** or higher
- **SQLite** (usually included with Go)
- **Redis** (required for session storage)
- **Tesseract OCR** (optional, for document text extraction)

### Development Environment Setup

We provide automated scripts to install all development dependencies:

#### For Debian/Ubuntu Systems:
```bash
./install-dev-deps-debian.sh
```

#### For Fedora Systems:
```bash
./install-dev-deps-fedora.sh
```

These scripts will install:
- Go 1.24.3
- Redis server
- Tesseract OCR with language packs
- All necessary development tools

### Building the Application

#### Build All Components
```bash
./build-all.sh
```

#### Build Individual Components
```bash
# Build only the Web UI
./build-webui.sh

# Build only the CLI
./build-cli.sh
```

#### Build Options
```bash
# Build with debug symbols
./build-all.sh --debug

# Build without Tesseract OCR support
./build-all.sh --notesseract

# Build with both options
./build-all.sh --debug --notesseract
```

After building, binaries will be available in the `bin/` directory:
- `bin/webui` - Web server
- `bin/ffcli` - Command-line interface

## ⚙️ Configuration & Setup

### Initial Setup

1. **Configure the application** using the interactive setup:
   ```bash
   ./bin/ffcli setup
   ```
   
   This will prompt you for configuration values and generate a configuration script.

2. **Apply the configuration**:
   ```bash
   # Linux/macOS
   source frozenfortress-config.sh
   
   # Windows
   frozenfortress-config.bat
   ```

3. **Create your first user**:
   ```bash
   ./bin/ffcli user create <username> <password>
   ```

4. **Activate the user**:
   ```bash
   ./bin/ffcli user activate <username>
   ```

### Environment Variables

FrozenFortress uses environment variables for configuration. All available variables:

| Variable | Description | Default |
|----------|-------------|---------|
| `FF_DATABASE_PATH` | Path to SQLite database | `~/.config/frozenfortress/frozenfortress.db` |
| `FF_MAX_SIGN_IN_ATTEMPTS` | Maximum sign-in attempts before account lockout | `3` |
| `FF_SIGN_IN_ATTEMPT_WINDOW` | Time window in minutes for counting sign-in attempts | `30` |
| `FF_REDIS_ADDRESS` | Redis server address | `localhost:6379` |
| `FF_REDIS_USER` | Redis username (leave empty if not required) | `""` |
| `FF_REDIS_PASSWORD` | Redis password (leave empty if not required) | `""` |
| `FF_REDIS_SIZE` | Redis connection pool size | `10` |
| `FF_REDIS_NETWORK` | Redis network type (tcp/unix) | `tcp` |
| `FF_SIGNING_KEY` | Session signing key (leave empty to auto-generate) | `""` |
| `FF_ENCRYPTION_KEY` | Session encryption key (leave empty to auto-generate) | `""` |
| `FF_KEY_DIR` | Directory to store persistent key files (empty = OS-specific user data dir) | `""` |
| `FF_WEB_UI_PORT` | Web UI server port | `8080` |
| `FF_LOG_LEVEL` | Log level (Debug, Info, Warn, Error) | `Info` |
| `FF_BACKUP_ENABLED` | Enable automatic backups | `false` |
| `FF_BACKUP_INTERVAL_DAYS` | Backup interval in days (0 = disabled) | `7` |
| `FF_BACKUP_DIRECTORY` | Directory where backup files are stored | `~/.config/frozenfortress/backups` |
| `FF_BACKUP_MAX_GENERATIONS` | Maximum number of backup files to keep | `10` |
| `FF_OCR_ENABLED` | Enable OCR functionality | `true` |
| `FF_OCR_LANGUAGES` | OCR languages (comma-separated, e.g., "eng,deu") | `eng` |

**Note:** When `FF_KEY_DIR` is empty (default), the system automatically uses OS-specific user data directories:
- **Linux**: `$XDG_CONFIG_HOME/frozenfortress` or `~/.config/frozenfortress`
- **Windows**: `%APPDATA%/frozenfortress` or `%LOCALAPPDATA%/frozenfortress`

Use `./bin/ffcli setup --read` to view current configuration values.

### Running the Application

#### Web Interface
```bash
# Run the web server
./run-webui.sh

# Or run directly
./bin/webui
```

The web interface will be available at `http://localhost:8080` (or your configured port).

#### CLI Administration
```bash
# View available commands
./bin/ffcli --help

# User management
./bin/ffcli user create <username> <password>
./bin/ffcli user activate <username>
./bin/ffcli user list
./bin/ffcli user lock <username>
./bin/ffcli user unlock <username>

# Backup management
./bin/ffcli backup create
./bin/ffcli backup list
./bin/ffcli backup cleanup

# Secret management
./bin/ffcli secret create <name> <value>
./bin/ffcli secret list
```

## 👩🏻‍💻 Web User Interface (WebUI)

The WebUI provides a modern, intuitive interface for end-users to manage their secrets and documents. It's designed for daily use and includes all the features needed for personal data management.

### Key Features
- **User Authentication**: Secure login with session management
- **Secret Management**: Create, edit, view, and organize secrets
- **Document Management**: Upload, view, and organize documents with OCR support
- **Search & Filter**: Advanced search capabilities across secrets and documents
- **Tag System**: Organize content with customizable tags
- **Account Management**: User profile and account settings
- **Responsive Design**: Works on desktop and mobile devices

### Available Views
- **Login/Registration**: User authentication and account creation
- **Secrets**: Manage passwords, API keys, and other sensitive information (main view after login)
- **Documents**: Upload, view, and manage documents with OCR text extraction
- **Tags**: Organize and manage tags for better content organization
- **Account Settings**: User profile management and security settings
- **Recovery**: Account recovery functionality

### User Workflow
FrozenFortress follows a secure user registration and activation workflow:

1. **User Registration**: New users can request access through the WebUI registration form
2. **Admin Activation**: An administrator with server access uses the CLI to activate the new user account
3. **User Login**: Once activated, users can log in and start using the application

Alternatively, administrators can create users directly via the CLI without requiring web registration.

## 🌐 Deployment & Security Considerations

FrozenFortress is specifically designed for **local self-hosting** environments. The application architecture prioritizes simplicity and ease of deployment while maintaining security best practices.

### Network Security
- **HTTP by Design**: The Go application runs on plain HTTP and does not include built-in HTTPS support
- **Reverse Proxy Architecture**: The application is intended to be deployed behind a reverse proxy (such as **nginx**) that handles HTTPS termination
- **Local Network Focus**: Designed primarily for local network deployment rather than direct internet exposure

### Recommended Deployment Setup
```
Local Network → nginx (HTTPS) → FrozenFortress WebUI (HTTP)
```

### Example nginx Configuration
```nginx
server {
    listen 443 ssl;
    server_name your-domain.com;
    
    ssl_certificate /path/to/your/certificate.crt;
    ssl_certificate_key /path/to/your/private.key;
    
    location / {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

### Security Best Practices
- **Use HTTPS**: Always deploy with HTTPS in production via reverse proxy
- **Firewall Rules**: Configure firewall to restrict access to the Go application port (default 8080)
- **Local Network**: Consider restricting access to local network ranges
- **Regular Updates**: Keep nginx and SSL certificates up to date

## 🖲️ Command Line Interface (CLI)

The CLI is designed for system administrators and provides comprehensive user management, backup operations, and system maintenance capabilities. It requires direct server access and operates with elevated privileges.

### ⚠️ Security & Access Control

**Important:** The CLI application is intended for **privileged administrators only** and should be treated as a high-privilege system tool.

- **Administrative Access**: CLI users are assumed to be system administrators with privileged access to the host system
- **No Authentication for Admin Tasks**: Administrative operations (user management, backups, configuration) do not require authentication
- **Authentication Required for User Data**: Operations involving encrypted user data require user password authentication
- **Direct Database Access**: CLI operations bypass web-based security controls and operate directly on the database
- **Elevated Privileges**: CLI users can create, modify, delete, and manage all user accounts and system data

### 🔐 Encryption Boundaries

**Critical:** Even with CLI administrative access, **encrypted user data remains protected** by user-specific encryption.

- **Encrypted Data Protection**: Administrators cannot access encrypted secrets, documents, or sensitive user data
- **User Password Required**: Any operations involving encryption/decryption require authentication with the user's password
- **Master Encryption Keys (MEK)**: User-specific encryption keys are derived from user passwords and cannot be bypassed
- **Admin Limitations**: CLI admins can manage user accounts but cannot decrypt or access user's encrypted content
- **Zero-Knowledge Architecture**: The system is designed so that even administrators cannot access user data without explicit user authentication

**Example:** An admin can activate/deactivate a user account, but cannot read that user's encrypted passwords or documents.

### Access Restrictions
- **File System Permissions**: Ensure CLI binary has appropriate file system permissions (e.g., executable only by admin users)
- **Server Access**: CLI should only be accessible to users with SSH/console access to the server
- **Network Isolation**: CLI does not expose network interfaces and requires local system access
- **Audit Trail**: Consider logging CLI usage for security auditing purposes

### Available Commands

#### User Management (`ffcli user`)
- `create <username> <password>` - Create a new user account
- `activate <username>` - Activate a user account (enables login)
- `deactivate <username>` - Deactivate a user account (disables login)
- `lock <username>` - Lock a user account (temporary suspension)
- `unlock <username>` - Unlock a locked user account
- `list` - List all user accounts with their status
- `delete <username>` - Permanently delete a user account

#### Secret Management (`ffcli secret`)
- `create <name> <value>` - Create a new secret
- `list` - List all secrets
- `delete <name>` - Delete a secret

#### Backup Management (`ffcli backup`)
- `create` - Create a manual backup of the database
- `list` - List all available backups
- `cleanup` - Remove old backups according to retention policy
- `delete <backup-id>` - Delete a specific backup

#### System Configuration (`ffcli setup`)
- `setup` - Interactive configuration wizard
- `setup --read` - Display current configuration values

#### Global Options
- `--verbose` - Enable verbose logging output
- `--help` - Show help information for any command

### CLI Usage Examples
```bash
# Create and activate a new user
./bin/ffcli user create john_doe SecurePassword123!
./bin/ffcli user activate john_doe

# List all users and their status
./bin/ffcli user list

# Create a backup
./bin/ffcli backup create

# View current configuration
./bin/ffcli setup --read

# Get help for any command
./bin/ffcli user --help
./bin/ffcli backup --help
```

## �🔧 Additional Scripts

- `./run-webui.sh` - Build and run the web UI
- `./stop-webui.sh` - Stop running web UI processes
- `./clean.sh` - Clean build artifacts

## 📦 Dependencies

### Core Dependencies
- `github.com/mattn/go-sqlite3` - SQLite driver
- `github.com/gin-gonic/gin` - Web framework
- `github.com/spf13/cobra` - CLI framework
- `github.com/gorilla/sessions` - Session management
- `golang.org/x/crypto` - Cryptographic functions
- `github.com/otiai10/gosseract/v2` - Tesseract OCR bindings
- `github.com/ledongthuc/pdf` - PDF processing

### Required External Services
- **Redis server** - Required for session storage and management

## 🔒 Security Features

- **Data Encryption**: All sensitive data is encrypted at rest
- **Secure Sessions**: Session-based authentication with secure cookies
- **Master Encryption Key (MEK)**: User-specific encryption keys
- **Password Hashing**: Secure password hashing with salts
- **Account Lockout**: Protection against brute force attacks
- **Recovery Codes**: Secure account recovery mechanism

## 📄 License

This project is licensed under the **MIT License**. See the LICENSE file for details.

## 🤝 Contributing

We welcome contributions! Please feel free to submit issues, feature requests, or pull requests.

### Development Workflow
1. Install development dependencies using the provided scripts
2. Make your changes
3. Build and test: `./build-all.sh`
4. Run tests and verify functionality
5. Submit a pull request
