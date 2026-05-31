# Deploy Control

## Current Status: `DEVELOPMENT`

deployctl is a cli tool to quickly create and run deployments

## Run

```sh
go run .
```

## Build

```sh
go build -o deployctl .
```

Deployment-name arguments complete from your saved deployments.

## Deployment flow

Create a deployment from a repository without building images immediately. This lets you configure env files or variables before the first build:

```sh
deployctl create https://github.com/owner/repo.git --name my-deployment
deployctl env set my-deployment APP_TAG=1.2.3
deployctl build my-deployment
deployctl deploy my-deployment
```

`deploy` starts the deployment with the existing local build. If every service is already running, deployctl reports the running containers and their Docker status instead of running Compose again. If the Compose build image is already cached, deployctl prints the cached image tag it will use. If no cached build exists, deployctl asks whether to build before starting.

You can also build as part of deploy when you want a fresh image:

```sh
deployctl deploy my-deployment --build
```

## Updating deployments

Pull the latest changes for a saved deployment without rebuilding images:

```sh
deployctl update my-deployment
```

Pull and rebuild in one command when you want the new source reflected in the local image:

```sh
deployctl update my-deployment --build
```

## Environment variables

Import an env file when you create a deployment:

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

When no env file is specified, `env set`, `env list`, and `env unset` use the deployment's default `.env` file.
This is ideal for basic compose env setups like:

```yaml
services:
  app:
    env_file:
      - .env
```

Compose also reads this default `.env` file when it interpolates values in the Compose file itself. If your Compose file uses an env variable for an image tag, build arg, port, volume, or other Compose field, set that variable in the default `.env` setup:

```yaml
services:
  app:
    image: ghcr.io/owner/app:${APP_TAG}
```

```sh
deployctl env set my-deployment APP_TAG=1.2.3
deployctl deploy my-deployment
```

Do not put Compose interpolation variables only in a service-specific `env_file` such as `app.env`. Service `env_file` entries are passed to the container environment after Compose has already resolved fields like `image: ...:${APP_TAG}`. Variables needed by the Compose file must be present in the deployment's default `.env` file, either with `deployctl create --env-file .env`, `deployctl env set my-deployment .env`, or `deployctl env set my-deployment APP_TAG=1.2.3`.

For Compose files with multiple service env files, pass the env file path exactly as it appears in the Compose file:

```yaml
services:
  my-app:
    env_file:
      - app.env
  backend:
    env_file:
      - backend.env
```

```sh
deployctl env set my-deployment app.env APP_PORT=8080 DEBUG=false
deployctl env set my-deployment backend.env ./local-backend.env
deployctl env list my-deployment app.env
deployctl env unset my-deployment backend.env DATABASE_URL
```

`env list` only shows variable names and masks values as `*****`.

## Private repositories

deployctl clones repositories by running `git clone`, so it uses the same local Git, SSH, and credential configuration as cloning manually in your terminal.

For HTTPS repository URLs, authenticate Git with GitHub CLI or Git Credential Manager before running deployctl:

```sh
gh auth login
gh auth setup-git
deployctl create https://github.com/owner/repo.git
```

For SSH repository URLs, deployctl uses your on-device SSH configuration through Git, including your SSH agent and `~/.ssh/config`:

```sh
deployctl create git@github.com:owner/repo.git
```

## Shell completion

deployctl can generate shell completion scripts. For zsh:

```sh
deployctl completion zsh > "${fpath[1]}/_deployctl"
```

Then restart your shell, or run:

```sh
exec zsh
```
