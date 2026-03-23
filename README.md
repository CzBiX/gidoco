# Gidoco (Git Docker Compose)

Gidoco is a lightweight Webhook receiver service designed to automate the deployment and management of Docker Compose projects by listening for Git repository updates.

Upon receiving a Webhook request, Gidoco automatically executes `git pull` in the local Git repository, analyzes the change history since the last update, and automatically performs complete updates and deployments in subdirectories where changes occurred (and which contain Docker Compose files). It also natively integrates support for [SOPS](https://github.com/getsops/sops), allowing it to automatically decrypt secret files in the repository.

## ✨ Features

- **Automated Triggering**: Provides a Webhook interface to receive push event signals in real-time.
- **Intelligent Update Mechanism**: Executes Compose operations only on directories affected by Git changes (by comparing `HEAD` Hash changes), saving system resources.
- **SOPS Automatic Decryption**: Built-in native support for decrypting secret files. By default, it automatically scans and decrypts changed `.enc.*` files (e.g., `secrets.enc.yml`).
- **Official Docker Integration**: Uses the Docker SDK to directly invoke instructions similar to `docker compose up` via code, avoiding environment compatibility issues caused by relying on external scripts.
- **Out-of-the-Box Configuration**: Supports flexible configuration via environment variables or a `config.yml` file.

## ⚙️ Configuration Guide

Gidoco supports reading configurations via environment variables or a `config.yml` file located in the working directory.

| Configuration Item | Environment Variable | Type | Default Value | Description |
| --- | --- | --- | --- | --- |
| `repoRoot` | `REPO_ROOT` | string | **Required** | The absolute path to the local Git repository managed by Gidoco |
| `port` | `PORT` | int | `8080` | The listening port for the Web service |
| `webhookId` | `WEBHOOK_ID` | string | - | If configured (e.g., `secret123`), the Webhook URL will become `/webhook/secret123`, serving as basic security protection |
| `encryptionEnabled`| `ENCRYPTION_ENABLED`| bool | `true` | When enabled, it will scan changed files for `*enc.*` and automatically invoke SOPS for decryption |
| `gitUrl` | `GIT_URL` | string | - | The remote Git URL. **Required** when `RepoRoot` is empty (auto-clone mode); otherwise defaults to the URL of the `origin` remote |
| `gitToken` | `GIT_TOKEN` | string | - | The access Token required for pulling code via HTTP/HTTPS protocols |
| `gitSshKey` | `GIT_SSH_KEY` | string | - | The private key content required for SSH protocol access |
| `gitSshKeyFile` | `GIT_SSH_KEY_FILE` | string | - | The private key file path required for SSH protocol access (mutually exclusive with `GitSshKey`) |
| `noStartUp` | `NO_START_UP` | bool | `false` | Do not run compose up for all projects on startup |
| `debug` | `DEBUG` | bool | `false` | Enables Debug mode |
| `dryRun` | `DRY_RUN` | bool | `false` | Enables DryRun mode |

## 💡 Usage

1. **Configure**: Set `REPO_ROOT` to the local path for the repository.
   > [!NOTE]
   > If the directory is empty or does not exist yet, also set `GIT_URL` along with the auth credentials (`GIT_TOKEN` or `GIT_SSH_KEY`).
   >
   > Gidoco will clone it automatically on first startup and immediately run `compose up` for all discovered Compose projects.
2. **Start Gidoco**: Run gidoco via binary or docker compose.
3. **Configure Webhook**: Set up a Webhook in GitHub / GitLab / Gitea:
   - Payload URL: `http://ServerIP:8080/webhook` (or `/webhook/:WebhookId` if configured).
   - Trigger condition: `Push Events`.
4. **Fully Automated Deployment**: From now on, every push triggers Gidoco to pull changes and update only the affected Compose projects.
