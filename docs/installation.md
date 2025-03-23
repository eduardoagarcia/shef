## Installation

### Homebrew

```bash
brew tap eduardoagarcia/tap
brew install shef

shef -v
```

### Binary

For Linux and Windows, the simplest way to install Shef is to download the pre-built binary for your platform from
the [latest release](https://github.com/eduardoagarcia/shef/releases/latest).

#### Linux / macOS

```bash
# Download the appropriate tarball for your platform
curl -L https://github.com/eduardoagarcia/shef/releases/latest/download/shef_[PLATFORM]_[ARCH].tar.gz -o shef.tar.gz

# Extract the binary
tar -xzf shef.tar.gz

# Navigate to the extracted directory
cd shef_[PLATFORM]_[ARCH]

# Make the binary executable
chmod +x shef

# Move the binary to your PATH
sudo mv shef /usr/local/bin/

# Verify installation
shef -v

# Sync public recipes
shef sync
```

Replace `[PLATFORM]` with `linux` or `darwin` (for macOS) and `[ARCH]` with your architecture (`amd64`, `arm64`).

#### Windows

1. Download the appropriate Windows ZIP file (`shef_windows_amd64.zip` or `shef_windows_arm64.zip`) from
   the [releases page](https://github.com/eduardoagarcia/shef/releases/latest)
2. Extract the archive using Windows Explorer, 7-Zip, WinRAR, or similar tool
3. Move the extracted executable to a directory in your PATH
4. Open Command Prompt or PowerShell and run `shef -v` to verify installation
5. Run `shef sync` to download recipes

### Verify Binaries

> [!IMPORTANT]
> Always verify the integrity of downloaded binaries for security. The release assets are signed with GPG, and you can
> verify them
using [the public key found in the repository](https://raw.githubusercontent.com/eduardoagarcia/shef/main/keys/shef-binary-gpg-public-key.asc).

#### Linux / macOS

```bash
# Import the GPG public key
curl -L https://raw.githubusercontent.com/eduardoagarcia/shef/main/keys/shef-binary-gpg-public-key.asc | gpg --import

# Download the signature file
curl -L https://github.com/eduardoagarcia/shef/releases/latest/download/shef_[OS]_[ARCH].tar.gz.asc -o shef.tar.gz.asc

# Verify the tarball
gpg --verify shef.tar.gz.asc shef.tar.gz
```

#### Windows

```bash
# Import the GPG public key (requires GPG for Windows)
Invoke-WebRequest -Uri "https://raw.githubusercontent.com/eduardoagarcia/shef/main/keys/shef-binary-gpg-public-key.asc" -OutFile "shef-key.asc"
gpg --import shef-key.asc

# Download the signature file
Invoke-WebRequest -Uri "https://github.com/eduardoagarcia/shef/releases/latest/download/shef_windows_amd64.zip.asc" -OutFile "shef_windows_amd64.zip.asc"

# Verify the ZIP file
gpg --verify shef_windows_amd64.zip.asc shef_windows_amd64.zip
```

### Package Managers

> [!NOTE]
> Future package manager support:
> - APT (Debian/Ubuntu)
> - YUM/DNF (RHEL/Fedora)
> - Arch User Repository (AUR)
> - Chocolatey/Scoop (Windows)

### Manual Installation

For developers or users who prefer to build from source:

```bash
git clone https://github.com/eduardoagarcia/shef.git
cd shef

# Install (requires sudo for system-wide installation)
make install

shef -v
```
