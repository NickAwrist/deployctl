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

## Shell completion

deployctl can generate shell completion scripts. For zsh:

```sh
deployctl completion zsh > "${fpath[1]}/_deployctl"
```

Then restart your shell, or run:

```sh
exec zsh
```

Deployment-name arguments complete from your saved deployments.

## Environment variables

Import an env file when you create a deployment:

```sh
deployctl create https://github.com/owner/repo.git --env-file .env
```

Manage env variables after creation:

```sh
deployctl env set my-deployment ENV_VARIABLE_ONE=123 ENV_VARIABLE_TWO=234
deployctl env list my-deployment
deployctl env unset my-deployment ENV_VARIABLE_ONE
```

`env list` only shows variable names and masks values as `*****`.

For Compose files, you can either use the common `.env` convention:

```yaml
services:
  app:
    env_file:
      - .env
```

Or make the deployctl file explicit while keeping a local fallback:

```yaml
services:
  app:
    env_file:
      - ${DEPLOYCTL_ENV_FILE:-.env.production}
```

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
