# CLI Installer

A utility for assisting projects in setting up a blank server with fxManager installed.

## Using the FXManager Installer

The `fxmanager-installer` CLI tool automates the setup of a FXServer (FiveM/RedM) instance. You can configure the installation path, target operating system, CFX license key, and txAdmin recipe (not yet implemented) using command-line flags.

### Basic Syntax

```bash
fxmanager-installer [flags]
```

### Available Flags

| Flag | Description | Default |
| --- | --- | --- |
| `-dir` | Target directory to set up the server. | `.` (current directory) |
| `-os` | Target operating system (`windows` or `linux`). | *Autodetects current OS* |
| `-license` | CFX license key to inject into `server.cfg` (get one the [Portal]([https:](https://portal.cfx.re)). | *None* |
| `-recipe` | GitHub repository URL for a txAdmin recipe. | *None* |

---

### Examples

**Minimal setup** (installs in the current directory using the autodetected OS):

```bash
fxmanager-installer
```

**Custom directory and license injection:**

```bash
fxmanager-installer -dir ./my-fivem-server -license cfxk_YOUR_LICENSE_KEY
```

**Full automated setup** (specifying the OS, injecting a license, and pulling a specific txAdmin recipe):

```bash
fxmanager-installer -dir ./myserver -os linux -license cfxk_YOUR_LICENSE_KEY -recipe https://github.com/overextended/txAdminRecipe
```

## Build the project locally

```bash
# windows
go build -o build/fxmanager-installer.exe .
# linux
go build -o build/fxmanager-installer .
```
