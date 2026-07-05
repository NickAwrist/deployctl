# deployctl

deployctl manages Docker Compose deployments through a local daemon.

## Requirements

- Linux with systemd.
- Git.
- Docker Engine available to the deployctl service user.
- Go 1.25+ or Docker for installing/building.
- A Docker Compose project to deploy.

## Install

```sh
curl -fsSL 'https://github.com/NickAwrist/deployctl/blob/main/scripts/install-linux.sh?raw=1' | bash
```

The installer builds `deployctl` and `deployctld`, installs them, writes a
systemd service, starts `deployctld`, and finishes with `deployctl daemon
status`.

Before installing, the script checks Git as the service user and stops if Git is
missing or unusable. It also tries to read the Docker server/API version with the
Docker CLI for install-time logging. If that Docker check cannot run, the
installer prints that Docker could not be validated and continues; if it can read
the API version and it is older than `MIN_DOCKER_API_VERSION` (default `1.40`),
the installer stops.

Run the same command again to update. Set `DEPLOYCTL_REF` to install a branch,
tag, or commit:

```sh
curl -fsSL 'https://github.com/NickAwrist/deployctl/blob/main/scripts/install-linux.sh?raw=1' | DEPLOYCTL_REF=1f0c104 bash
```

Without `sudo`, the installer creates a user service and installs to
`~/.local/bin`. For a system service:

```sh
curl -fsSL 'https://github.com/NickAwrist/deployctl/blob/main/scripts/install-linux.sh?raw=1' | sudo env DEPLOYCTL_USER=deploy bash
```

The service user needs Docker access plus Git credentials and SSH keys for any
private repositories it deploys.

To uninstall:

```sh
curl -fsSL 'https://github.com/NickAwrist/deployctl/blob/main/scripts/uninstall-linux.sh?raw=1' | bash
```

Use `sudo` for system-service uninstalls. To remove deployment state too:

```sh
curl -fsSL 'https://github.com/NickAwrist/deployctl/blob/main/scripts/uninstall-linux.sh?raw=1' | REMOVE_DATA=1 bash
```

## Build

```sh
make test
make generate
make build
```

`make build` writes binaries to `bin/`. `make generate` updates the protobuf
and gRPC stubs from `api/deployctl/v1/deployctl.proto`.

## Daemon

The installer starts `deployctld` automatically unless `SKIP_SERVICE=1` is set.

```sh
deployctl daemon status
deployctl daemon restart
```

Use `deployctl daemon start` only to run a foreground daemon manually. The CLI
talks to the daemon over a Unix socket; override it with `DEPLOYCTL_SOCKET_PATH`.
Daemon logs default to `~/.deployctl/deployctld.log`; override with
`DEPLOYCTL_LOG_PATH`.

Deployment commands run as daemon jobs. Pass `--detach` to start a job and
return immediately.

## Deployment Flow

```sh
deployctl create https://github.com/owner/repo.git --name my-deployment
deployctl env set my-deployment APP_TAG=1.2.3
deployctl build my-deployment
deployctl deploy my-deployment
```

`deploy` uses an existing build when available. Add `--build` to force a rebuild:

```sh
deployctl deploy my-deployment --build
deployctl restart my-deployment --build
deployctl update my-deployment --build
```

Common commands:

```sh
deployctl list
deployctl status my-deployment
deployctl restart my-deployment
deployctl stop my-deployment
deployctl update my-deployment
deployctl delete my-deployment
```

`deployctl status` shows container state, image details, recent job timing, the
latest update job, and env variable names with values masked.

## Environment

Import an env file when creating a deployment:

```sh
deployctl create https://github.com/owner/repo.git --env-file .env
```

Manage env variables after creation:

```sh
deployctl env set my-deployment ENV_VARIABLE_ONE=123 ENV_VARIABLE_TWO=234
deployctl env set my-deployment .env
deployctl env list my-deployment
deployctl env unset my-deployment ENV_VARIABLE_ONE
```

When no env file is specified, `env set`, `env list`, and `env unset` use the
deployment's default `.env` file. Put Compose interpolation values there:

```yaml
services:
  app:
    image: ghcr.io/owner/app:${APP_TAG}
    env_file:
      - .env
```

```sh
deployctl env set my-deployment APP_TAG=1.2.3
deployctl deploy my-deployment
```

For service-specific env files, pass the path exactly as it appears in the
Compose file:

```sh
deployctl env set my-deployment app.env APP_PORT=8080 DEBUG=false
deployctl env list my-deployment app.env
deployctl env unset my-deployment app.env DATABASE_URL
```

`env list` only shows variable names and masks values as `*****`.

## Private Repositories

deployctl runs Git directly, so repository access uses the service user's Git,
SSH, and credential configuration.

HTTPS:

```sh
gh auth login
gh auth setup-git
deployctl create https://github.com/owner/repo.git
```

SSH:

```sh
deployctl create git@github.com:owner/repo.git
```

## Shell Completion

For zsh:

```sh
deployctl completion zsh > "${fpath[1]}/_deployctl"
exec zsh
```
