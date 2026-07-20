# CLI Installer

A utility for assisting projects in setting up or updating a server with **fxManager** installed.

---

> [!NOTE]
> **Action Support:** The installer currently supports fresh installations. The **Update** workflow is under active development and marked as **WIP (Coming Soon)** in the interactive menu.
> 
> **Recipe Installer:** The recipe installer is not currently available for use.

---

## Features

* **Interactive Setup Wizard:** Parameters are fully optional. If skipped, an interactive TUI prompt guides you through configuring your setup step-by-step.
* **Automated Scripting:** Pass command-line flags to pre-fill or entirely bypass interactive prompts for automated deployments.
* **Environment Provisioning:** Downloads the latest FXServer artifacts, scaffolds file structures, configures `server.cfg`, and manages `fxManager` resources.

---

## Quick Install & Run

Run a single command in your terminal to download the latest executable as `fxmanager-installer` and immediately launch it:

### Windows (PowerShell)

```bash
Invoke-WebRequest -Uri \
    'https://github.com/fxManagerProject/cli-installer/releases/latest/download/fxmanager-installer-windows-amd64.exe' \
    -OutFile 'fxmanager-installer.exe'; \
    .\fxmanager-installer.exe
```

### Linux

```bash
curl -sSL \
    'https://github.com/fxManagerProject/cli-installer/releases/latest/download/fxmanager-installer-linux-amd64' \
    -o fxmanager-installer && \
    chmod +x fxmanager-installer && \
    ./fxmanager-installer
```

---

## Usage

### Interactive Mode (Default)

Simply run the executable with no arguments to launch the interactive prompt wizard:

```bash
fxmanager-installer
```

You will be presented with an interactive menu to select your action (**Install** or **Update**) and configure any missing specifics (target directory, operating system, license key, and optional recipes).

---

### Non-Interactive / Unattended Mode

You can supply configuration values ahead of time using flags. Any omitted flags will either fallback to defaults or prompt interactively if required.

```bash
fxmanager-installer [flags]
```

### Available Flags

| Flag | Description | Default / Fallback |
| --- | --- | --- |
| `-dir` | Target directory where the server will be set up. | `.` *(Current Directory)* |
| `-os` | Target operating system (`windows` or `linux`). | *Autodetects host OS* |
| `-license` | CFX license key to inject into `server.cfg` (get one from [cfx.re Portal](https://portal.cfx.re)). | *Prompts interactively* |
| `-recipe` | GitHub repository URL or txAdmin recipe source. | *None (Optional)* |

---

### Examples

**1. Launch interactive menu:**

```bash
fxmanager-installer
```

**2. Direct install with target directory and CFX license pre-filled:**

```bash
fxmanager-installer -dir ./my-fivem-server -license cfxk_YOUR_LICENSE_KEY
```

**3. Fully unattended / automated installation:**

```bash
fxmanager-installer \
  -dir ./myserver \
  -os linux \
  -license cfxk_YOUR_LICENSE_KEY \
  -recipe https://github.com/overextended/txAdminRecipe
```

---

## Building Locally

To compile the binary locally for your platform:

```bash
# Windows
go build -o build/fxmanager-installer.exe .

# Linux / macOS
go build -o build/fxmanager-installer .
```
