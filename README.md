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
