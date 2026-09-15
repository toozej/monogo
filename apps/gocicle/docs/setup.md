# Set up a new Gocicle instance

This guide installs a Linux control plane as a systemd service. Run repository-relative commands from the monogo root. Replace example domains, paths, and `REPLACE_ME` values before use.

Complete these steps once for each instance:

1. Prepare PostgreSQL, the service account, and the encryption key.
2. Configure the public HTTPS origin and proxy.
3. Register a login provider.
4. Create the first administrator through an invitation and OAuth sign-in.
5. Register additional providers and approve users.
6. Enroll a runner and import a job.

The [Compose procedure](#development-compose) provides an alternative for local development. Browser sign-in requires HTTPS with either installation method.

## Prepare the control plane

Install PostgreSQL and a reverse proxy. The acceptance tests use PostgreSQL 17. Install the Gocicle binary from an app release, or build it with `make local-build APP=gocicle`.

The following commands assume a Debian-style host with `sudo`, systemd, and a local PostgreSQL administrator named `postgres`. Skip account and database creation if those resources already exist.

```sh
sudo useradd --system --user-group --home-dir /var/lib/gocicle --create-home \
    --shell /usr/sbin/nologin gocicle
sudo install -o root -g root -m 0755 out/gocicle /usr/local/bin/gocicle
sudo install -d -o gocicle -g gocicle -m 0700 /etc/gocicle
sudo -u postgres createuser --pwprompt gocicle
sudo -u postgres createdb --owner=gocicle gocicle
sudo -u gocicle /usr/local/bin/gocicle keygen --output /etc/gocicle/keys.json
sudo install -o gocicle -g gocicle -m 0600 \
    apps/gocicle/deploy/control.env.example /etc/gocicle/control.env
sudoedit /etc/gocicle/control.env
```

The database role must own the database objects that migrations create. Set PostgreSQL authentication to allow that role to connect. `keygen` creates a mode `0600` file and refuses to replace an existing file. Keep the key outside PostgreSQL. Back up the key separately from the database.

Set these values in `control.env`:

```sh
GOCICLE_DATABASE_URL='postgres://gocicle:REPLACE_ME@127.0.0.1:5432/gocicle?sslmode=require'
GOCICLE_KEY_FILE=/etc/gocicle/keys.json
GOCICLE_LISTEN=127.0.0.1:8080
GOCICLE_PUBLIC_URL=https://gocicle.example.com
GOCICLE_TRUSTED_PROXIES=127.0.0.1/32,::1/128
GOCICLE_RETENTION=720h
```

Use the database's actual TLS settings. The example requires PostgreSQL TLS. URL-encode reserved characters in the database password. Keep shell metacharacters inside quotes because the administrative helper below also reads this file as shell input.

`GOCICLE_PUBLIC_URL` must be an HTTPS origin without a trailing slash, path, query, or fragment. Use the same origin in the browser and forge registrations. Set `GOCICLE_TRUSTED_PROXIES` to the exact proxy address ranges. The loopback values above apply to a proxy on this host.

Define this helper in the administrative shell. It uses the service account and the same configuration as systemd:

```sh
gocicle_admin() {
    sudo -u gocicle bash -e -c '
        set -a
        . /etc/gocicle/control.env
        set +a
        exec /usr/local/bin/gocicle "$@"
    ' bash "$@"
}
gocicle_admin migrate
```

Run migrations before `serve`. Repeat the migration command when upgrading the binary. Startup only checks the schema version.

## Configure HTTPS and start the service

1. Point the chosen host name to the reverse proxy.
2. Configure either [nginx](../deploy/nginx.conf) or [Caddy](../deploy/Caddyfile) for that host name.
3. Configure a trusted TLS certificate.
4. Keep the examples' streaming settings and proxy all paths to `127.0.0.1:8080`.
5. Start the proxy with your host's service manager.
6. Install and start the control plane unit:

```sh
sudo install -m 0644 apps/gocicle/deploy/gocicle.service /etc/systemd/system/gocicle.service
sudo systemctl daemon-reload
sudo systemctl enable --now gocicle.service
sudo systemctl status gocicle.service
curl --fail --silent --show-error https://gocicle.example.com/api/v1/providers
```

The last command returns a JSON array. It can be empty before provider registration. The browser should show the sign-in page. Use `journalctl -u gocicle.service` to inspect startup errors.

Allow `/auth/*`, `/api/v1/providers`, and `/oauth-client-metadata.json` through the proxy. Preserve callback query parameters. An additional proxy login would prevent an AT Protocol authorization server from reading the public client metadata.

## Register login providers

Register an application on each forge that will authenticate users. Users then authorize that application during sign-in. Creating a repository credential does not register a login provider.

Gocicle requests the scopes below. Scope names come from the current adapter code. They are not configurable through `app.yaml` or provider JSON.

| Forge | `Kind` | `Config.baseURL` | Requested scopes |
|---|---|---|---|
| GitHub | `github` | `https://github.com` | `read:user` |
| GitLab | `gitlab` | `https://gitlab.com` or the instance HTTPS origin | `read_user read_api` |
| Codeberg | `codeberg` | `https://codeberg.org` | `read:user read:repository` |
| Forgejo | `forgejo` | The instance HTTPS origin | `read:user read:repository` |
| SourceHut | `sourcehut` | `https://meta.sr.ht` | `meta.sr.ht/PROFILE:RO git.sr.ht/REPOSITORIES:RO` |
| Tangled | `tangled` | `https://tangled.org` | `atproto` |
| Generic Git | `git` | No login origin | No OAuth sign-in |

Provider HTTPS requests reject private addresses and redirects. A configured GitLab or Forgejo instance must resolve to public addresses from the control plane. Use its canonical HTTPS origin. An internal-only forge will not work with the current provider policy.

### First provider through the local CLI

Use GitHub, GitLab, Codeberg, Forgejo, or [Tangled](#tangled) for the initial administrator sign-in. Tangled has a separate registration procedure. Add SourceHut after that sign-in with the [administrator procedure](#add-a-provider-from-an-administrator-session), which obtains the final callback before forge registration.

For the first four providers, complete these steps in order:

1. Create a forge OAuth application using the provider-specific settings below.
2. Initially enter `https://gocicle.example.com/auth/setup/callback` as its callback.
3. Copy the forge's client ID and client secret into a protected local JSON file.
4. Run `provider-add` to obtain Gocicle's provider ID.
5. Edit the forge application to replace the temporary callback with the final callback.
6. Attempt sign-in only after saving the final callback.

The temporary URL only reserves a value in the forge form. It cannot complete a Gocicle login. This sequence is necessary because `provider-add` generates a new provider ID. It does not accept an ID or update an existing record.

Create the protected file before entering credentials:

```sh
sudo install -o gocicle -g gocicle -m 0600 /dev/null /etc/gocicle/provider.json
sudoedit /etc/gocicle/provider.json
```

Example for GitHub:

```json
{
  "Name": "GitHub",
  "Kind": "github",
  "ClientSecret": "REPLACE_ME_CLIENT_SECRET",
  "Config": {
    "baseURL": "https://github.com",
    "clientID": "REPLACE_ME_CLIENT_ID"
  }
}
```

Use a unique `Name` for each provider instance. Change `Kind` and `baseURL` using the table above. Omit `secretID` and `redirectURL`. Gocicle encrypts `ClientSecret`, stores its reference, and derives the callback from `GOCICLE_PUBLIC_URL`.

```sh
if gocicle_provider_id=$(gocicle_admin provider-add /etc/gocicle/provider.json); then
    printf 'Final callback: https://gocicle.example.com/auth/%s/callback\n' "$gocicle_provider_id"
    sudo rm /etc/gocicle/provider.json
fi
```

Copy the printed final callback into the forge registration. `PROVIDER_ID` is Gocicle's returned ID, not `github` or the forge's client ID. Keep this record's ID when changing credentials later. Gocicle identifies accounts by provider record and stable provider subject.

### GitHub

1. Open **Settings > Developer settings > OAuth apps** on GitHub.
2. Create an OAuth App with a name that identifies this Gocicle instance.
3. Set **Homepage URL** to the Gocicle HTTPS origin.
4. Set **Authorization callback URL** according to the registration procedure you selected.
5. Register the application and generate a client secret.
6. Use **Client ID** as `clientID` and the generated secret as `ClientSecret`.

Gocicle uses the authorization-code flow. Leave device flow disabled. Complete the final callback replacement when using the local CLI procedure. See [GitHub's OAuth App registration instructions](https://docs.github.com/en/apps/oauth-apps/building-oauth-apps/creating-an-oauth-app).

Gocicle requests `read:user`. This does not grant private repository discovery or Git writes. Enter private clone URLs manually and configure separate repository credentials. Gocicle does not refresh stored discovery tokens automatically. Sign in again when discovery reports an expired or revoked token.

### GitLab

1. Open **Edit profile > Access > Applications** on the selected GitLab instance.
2. Add an application for this Gocicle instance.
3. Set its redirect URI according to the selected registration procedure.
4. Keep the application confidential when that option is available.
5. Enable `read_user` and `read_api`.
6. Save the application and copy its **Application ID** and **Secret**.

Use the Application ID as `clientID`. Set `Kind` to `gitlab`. Set `baseURL` to the same instance where you registered the application. The default API root is that origin plus `/api/v4`. Group-owned applications can use the same settings. See [GitLab's registration instructions](https://docs.gitlab.com/integration/oauth_provider/) and [confidential-client setting](https://docs.gitlab.com/api/user_applications/).

GitLab access tokens expire. Sign in again to replace an expired discovery token. Git clone and push credentials require separate project grants.

### Codeberg and Forgejo

1. Open **Settings > Applications** on Codeberg or the chosen Forgejo instance.
2. Create an OAuth2 application under `/user/settings/applications`.
3. Set its name and redirect URI for this Gocicle instance.
4. Select the confidential-client option when the form provides it.
5. Save the application and copy its client ID and client secret.
6. Use `codeberg` with `https://codeberg.org`, or `forgejo` with the chosen instance origin.

The default API root is the forge origin plus `/api/v1`. Complete the final callback replacement when using the local CLI procedure. [Forgejo documents this registration flow for both Forgejo and Codeberg](https://forgejo.org/docs/latest/user/authentication/oauth2-provider/).

Gocicle sends `read:user read:repository`, but Forgejo currently documents that OAuth2 scopes are not enforced. Do not interpret these requested scope names as a provider-enforced read-only restriction. Use the user application page for this procedure. Forgejo's instance-wide administrator applications have different access implications. See [Forgejo's OAuth2 scope limitations](https://forgejo.org/docs/latest/user/authentication/oauth2-provider/#git-authentication).

### SourceHut

Use an existing Gocicle administrator session and the procedure below to obtain the final callback before registering SourceHut. This avoids relying on later callback editing in SourceHut. The local `provider-add` command cannot update client credentials in an existing provider record.

1. Create the Gocicle provider record with kind `sourcehut` and base URL `https://meta.sr.ht`.
2. Copy its final callback URL.
3. Sign in to `meta.sr.ht` and open the OAuth2 client dashboard.
4. Register a confidential OAuth client with that callback, an instance name, and the Gocicle homepage URL.
5. Copy the client's UUID and registration secret.
6. Finish the administrator procedure with the UUID as `clientID` and the secret as an `oauth` credential.

SourceHut's [client registration schema](https://docs.sourcehut.org/meta.sr.ht/#registerOAuthClient) returns a client and a secret. Use the client's UUID, not its numeric database ID or a personal access token. The [SourceHut OAuth2 dashboard announcement](https://sourcehut.org/blog/2022-09-15-whats-cooking-september-2022/) identifies the account dashboard used for OAuth2 management.

Gocicle requests profile and repository read grants. Its SourceHut adapter uses `meta.sr.ht` for profile queries and `git.sr.ht` for repository queries. Use SSH credentials for repository writes. OAuth approval does not install an SSH key.

### Tangled

Gocicle uses a public AT Protocol OAuth client for Tangled. Its client ID is the HTTPS metadata URL. You do not obtain a client ID or client secret from a Tangled application-registration form. The authorization server retrieves the document during discovery. See [AT Protocol client metadata](https://atproto.com/specs/oauth#client-id-metadata-document).

Use the standard HTTPS port without an explicit port number in `GOCICLE_PUBLIC_URL`. AT Protocol client IDs cannot contain a port number. Gocicle does not implement the protocol's special localhost client mode.

Create this provider file using the protected-file commands above:

```json
{
  "Name": "Tangled",
  "Kind": "tangled",
  "Config": {"baseURL": "https://tangled.org"}
}
```

```sh
gocicle_admin provider-add /etc/gocicle/provider.json && sudo rm /etc/gocicle/provider.json
curl --fail --silent --show-error https://gocicle.example.com/oauth-client-metadata.json
```

Check that the document contains:

- `client_id`: `https://gocicle.example.com/oauth-client-metadata.json`.
- `redirect_uris`: the callback for the returned Gocicle provider ID.
- `scope`: `atproto`.
- `token_endpoint_auth_method`: `none`.
- `dpop_bound_access_tokens`: `true`.

The authorization server must reach this document over public HTTPS without a login, redirect, or certificate exception. Gocicle supplies PKCE, PAR, and DPoP during sign-in. It discovers the authorization server from the user's Personal Data Server (PDS).

Select Tangled on the Gocicle sign-in page. Enter an AT Protocol handle, such as `user.bsky.social`, or a supported DID. Complete consent on the account provider. A hostname without an account handle is not a login hint. Tangled supports existing AT Protocol accounts, as described in its [sign-in guide](https://docs.tangled.org/single-page#login-or-sign-up).

For pushes, add the appropriate public SSH key to Tangled and configure a separate Gocicle SSH credential for the resolved repository. Follow [Tangled's SSH setup](https://docs.tangled.org/single-page#add-an-ssh-key). The OAuth client secret field remains empty.

### Generic Git

Generic Git has no OAuth application or browser sign-in. Use another listed provider for user authentication. Enter the repository clone URL manually. Configure HTTPS or SSH repository credentials only when the repository requires them.

## Create the first administrator

Finish the first provider's callback or metadata configuration before creating the invitation:

```sh
gocicle_admin bootstrap
```

1. Open the configured Gocicle HTTPS origin in a browser.
2. Select the configured provider.
3. Paste the printed invitation into **Invitation (first sign-in only)**.
4. Enter the account handle if you selected Tangled.
5. Select **Sign in** and approve the forge authorization request.
6. Confirm that Gocicle displays **Settings** with `admin/users` and `admin/providers` choices.

The invitation is single-use and expires after one hour. If it expires before use, run `bootstrap` again. That command invalidates the previous administrator invitation. It refuses to run after an administrator exists. Keep the invitation private until you consume it.

Sign-in without an invitation creates a disabled account. An existing administrator must approve it. If this happens before the first administrator exists, run `bootstrap` and repeat sign-in with its invitation.

## Add a provider from an administrator session

This procedure obtains Gocicle's callback before creating the forge application. It works for the confidential providers above, including SourceHut.

1. In **Settings**, select `admin/providers`.
2. Clear **Record ID** and **Current version**.
3. Enter the following JSON, with the chosen provider name, kind, and origin.
4. Select **Save settings**.

```json
{
  "name": "SourceHut",
  "kind": "sourcehut",
  "config": {"baseURL": "https://meta.sr.ht"}
}
```

Save the returned `id` and `version`. Register the forge application with `https://gocicle.example.com/auth/RETURNED_ID/callback` using the forge-specific instructions above. The provider cannot complete sign-in until the next steps supply its credentials.

Select `secrets` in Settings. Clear **Record ID** and **Current version**. Save this JSON with the actual client secret:

```json
{
  "name": "SourceHut OAuth",
  "kind": "oauth",
  "value": {"value": "REPLACE_ME_CLIENT_SECRET"}
}
```

Copy the returned secret `id`. Select `admin/providers` again. Set **Record ID** to the original provider ID. Set **Current version** to its saved version. Save the completed provider JSON:

```json
{
  "name": "SourceHut",
  "kind": "sourcehut",
  "config": {
    "baseURL": "https://meta.sr.ht",
    "clientID": "REPLACE_ME_CLIENT_UUID",
    "secretID": "REPLACE_ME_SECRET_ID"
  }
}
```

The browser settings use `config.secretID`, not the local CLI's `ClientSecret` field. Reload the page after adding a provider. To use the additional forge with your existing account, open **Link another login identity** while signed in. Complete the new provider's consent flow. Matching email addresses do not link accounts automatically.

For secret rotation, create a new `oauth` secret and update the same provider record's `config.secretID`. Use its current version. Keep the old forge secret valid until the replacement works, if the forge permits overlap. Do not rerun `provider-add` to rotate a secret: it creates a different provider record and callback.

## Approve users and run the first job

1. Have the user complete their first forge sign-in.
2. As administrator, select `admin/users` in Settings and select **Load settings**.
3. Find the intended disabled account and confirm its identity with the user.
4. Enter its `id` and current `version` in the corresponding fields.
5. Paste that single user object into the editor.
6. Set `enabled` to `true` and retain the intended `administrator` value.
7. Save the settings and have the user sign in again.

For the first job, follow [runner installation](../README.md#runner-installation) to enroll a Linux runner through Docker or Podman. Grant the project owner access to that runner. Enroll and grant each additional runner separately.

Create a project and configure [repository credentials and project grants](../README.md#credentials-and-preferences). OAuth sign-in does not authorize runner use, project membership, or repository writes. Configure the project's commit name and email before enabling pushes.

Follow [repository import](../README.md#projects-and-jobs) to read `.gocicle.yaml`, review the preview, and map credentials. Enable a validated job. Start a manual run and check its logs and result before relying on its schedule. If pushes are enabled, check the separate push result and the source branch.

## Development Compose

The [development Compose file](../deploy/compose.dev.yaml) provides persistent PostgreSQL storage and the distroless control plane. Set `GOCICLE_POSTGRES_PASSWORD`, `GOCICLE_PUBLIC_URL`, and an absolute `GOCICLE_KEY_FILE` path in the shell. Use a URL-safe database password because Compose inserts it into a connection URL.

Generate the key once with `gocicle keygen --output /absolute/path/keys.json`. The container runs as UID/GID `65532`. Ensure that identity owns the mounted key while retaining mode `0600`. Do not make the key world-readable. Host CLI commands must also run as an identity that can read the key.

```sh
docker compose -f apps/gocicle/deploy/compose.dev.yaml up -d postgres
docker compose -f apps/gocicle/deploy/compose.dev.yaml build gocicle
docker compose -f apps/gocicle/deploy/compose.dev.yaml run --rm gocicle migrate
docker compose -f apps/gocicle/deploy/compose.dev.yaml up -d gocicle
```

Configure an HTTPS proxy before browser sign-in. For CLI provider registration, mount the protected provider JSON read-only. Supply its absolute path through `GOCICLE_PROVIDER_FILE`. Ensure UID `65532` can read that file.

```sh
docker compose -f apps/gocicle/deploy/compose.dev.yaml run --rm \
    -v "$GOCICLE_PROVIDER_FILE:/run/secrets/provider.json:ro" \
    gocicle provider-add /run/secrets/provider.json
```

Complete the callback registration or Tangled metadata check. Then create the administrator invitation:

```sh
docker compose -f apps/gocicle/deploy/compose.dev.yaml run --rm gocicle bootstrap
```

Continue with the same browser steps. Install runners as host services. The control plane container does not contain the host Git, SSH, and runtime environment required by a runner.

## Diagnose sign-in problems

| Symptom | Check |
|---|---|
| No provider in the sign-in list | Run provider registration against the same database as the service. Reload the page. |
| Redirect URI mismatch | Use the exact public origin and Gocicle provider ID. Remove the temporary callback from the forge registration. |
| Invalid client or reconnect error | Check the application type, client ID, encrypted client secret, and forge instance. A PAT is not an OAuth client secret. |
| Callback returns forbidden | Use the same browser and HTTPS origin throughout sign-in. Start a new attempt after the ten-minute OAuth state expires. |
| Sign-in succeeds at the forge but Gocicle denies access | Supply the first administrator invitation, or have an administrator enable the user. |
| Provider request fails | Check public DNS, trusted TLS, outbound connectivity, and the canonical provider URL. Private addresses and HTTP redirects are rejected. |
| Tangled discovery or PAR fails | Check the public client metadata, handle/DID resolution, and the PDS authorization-server metadata. |
| Repository list stops working | Sign in again to replace expired discovery credentials. Automatic discovery-token refresh is not implemented. |
| A job cannot clone or push after sign-in | Configure the repository credential and its project grant. OAuth login and Git transport use separate credentials. |

Provider tests use local fixtures. A live sign-in on your configured instance is the final deployment check. If the public origin changes, update the service configuration and every forge callback. Restart the service and verify Tangled's metadata again.

Before scheduling production jobs, complete the [backup and restoration procedure](../README.md#operations) for both PostgreSQL and the external encryption keys.
