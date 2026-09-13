# Portainer deployment

This folder is the single Compose path for the Git-linked Portainer stack.
It uses official Multica images and adds only a small, fail-closed inline
entrypoint which loads the GitHub App PEM from an owner-only Docker-host file.

## Portainer settings

- Repository URL: `https://github.com/mARTin-B78/multica.git`
- Compose path: `portainer/docker-compose.yml`
- Git reference: `main`

Set the existing Multica environment variables in Portainer as before. For
the GitHub App, set `GITHUB_APP_SLUG`, `GITHUB_APP_ID`,
`GITHUB_WEBHOOK_SECRET`, and the non-secret absolute host path
`GITHUB_APP_PRIVATE_KEY_FILE`.

Do **not** set `GITHUB_APP_PRIVATE_KEY` in Portainer. Create the protected PEM
file on the Docker host, set its mode to `0600`, and make it readable by the
account that runs Docker. The wrapper rejects a flattened or malformed key.

The example environment file contains no secret values.

## Updating upstream

This repository is a private mirror, not a GitHub fork: GitHub keeps forks of
public repositories public. To update it, merge or rebase `upstream/main` into
`main`, resolve any deployment-file conflicts, test `docker compose config`,
then push `main`. Portainer will redeploy the selected Git reference.
