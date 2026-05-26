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

## Private repositories

For HTTPS repository URLs, set a token with repository access:

```sh
DEPLOYCTL_GIT_TOKEN=github_pat_... deployctl create https://github.com/owner/repo.git
```

For SSH repository URLs, deployctl uses your SSH agent, `~/.ssh/config` `IdentityFile` entries, and standard private key paths. You can also point deployctl at a specific private key:

```sh
DEPLOYCTL_SSH_KEY=~/.ssh/id_ed25519 deployctl create git@github.com:owner/repo.git
```
