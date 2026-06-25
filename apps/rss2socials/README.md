# rss2socials

![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/toozej/rss2socials)
[![Go Report Card](https://goreportcard.com/badge/github.com/toozej/monogo/apps/rss2socials)](https://goreportcard.com/report/github.com/toozej/monogo/apps/rss2socials)
![GitHub Actions Workflow Status](https://img.shields.io/github/actions/workflow/status/toozej/rss2socials/cicd.yaml)
![Docker Pulls](https://img.shields.io/docker/pulls/toozej/rss2socials)
![GitHub Downloads (all assets, all releases)](https://img.shields.io/github/downloads/toozej/rss2socials/total)

rss2socials is a CLI tool that monitors an RSS feed for new posts and automatically posts updates to specified social platforms (Mastodon, Bluesky, Threads). This application is designed for easy configuration and seamless integration.

## Features
- Periodically checks an RSS feed for new or updated posts.
- Posts updates to configured social platforms (Mastodon, Bluesky, Threads).
- Stores previously posted items in an SQLite database to avoid duplicates.
- **PostNewEntriesOnly** mode (default: enabled) prevents posting all existing RSS feed entries on first startup — only entries that appear after the first successful check are posted.
- Configurable check interval and customizable content.
- Debug mode for detailed logging.

## Installation
### Prerequisites
- Go (version 1.25 or later)
- SQLite for database management
- Make

### Steps
1.	Clone the repository:
```bash
git clone https://github.com/toozej/rss2socials.git
cd rss2socials
```

2.	Build the executable:
`make build`

## Usage
1.	Set Environment Variables:
    Create a .env file in the root of your project or set the required environment variables directly:

```
# Mastodon
MASTODON_URL=https://your-mastodon-instance
MASTODON_CLIENT_KEY=your-client-key
MASTODON_CLIENT_SECRET=your-client-secret
MASTODON_ACCESS_TOKEN=your-access-token

# Bluesky
BLUESKY_HANDLE=your.handle.bsky.social
BLUESKY_APPKEY=your-app-key
    
# Threads
THREADS_USER_ID=your-user-id
THREADS_ACCESS_TOKEN=your-access-token
THREADS_CLIENT_ID=your-client-id
THREADS_CLIENT_SECRET=your-client-secret
THREADS_REDIRECT_URI=https://yourapp.com/callback

# Optional: specify which social sites to post to (defaults to all with credentials configured)
# SOCIAL_SITES=mastodon,bluesky,threads

# General
FEED_URL=https://example.com/rss
POST_NEW_ENTRIES_ONLY=true

# Database
DB_PATH=/data/db/sqlite.db
```

    Alternatively, you can provide parameters as command-line flags.

2.	Run the application:
    ```bash
    ./rss2socials --feed-url "https://example.com/rss" --interval 60
    ```

`--feed-url`: The URL of the RSS feed to monitor.
`--interval`: The interval in minutes for checking the RSS feed (default is 60 minutes).
`--post-new-entries-only`: Only post entries that appear after first startup; existing feed entries are stored but not posted (default: true). Set to false to post all entries on first run.

3. Enable Debug Mode:
Use the --debug flag to enable debug-level logging for troubleshooting.
```bash
./rss2socials --debug
```


## Major Components
### Command Structure (cmd/rss2socials/root.go)
- Defines the main rss2socials command and its subcommands (man and version).
- Sets up CLI flags and binds them to configuration via Viper.

### Configuration (pkg/config/config.go)
- Loads configuration from environment variables and the .env file if present.

### RSS Handling (internal/rss/rss.go)
- Fetches and parses the RSS feed.
- Provides hashing functionality to detect changes in post content.

### Social Integrations
- **Mastodon**: `internal/mastodon`

#### Creating a Mastodon Application

To obtain the required `MASTODON_CLIENT_KEY`, `MASTODON_CLIENT_SECRET`, and `MASTODON_ACCESS_TOKEN` credentials:

1. Log in to your Mastodon instance (e.g., `https://mastodon.social`).
2. Go to **Preferences** → **Development** → **New Application**.
3. Fill in the application name (e.g., `rss2socials`).
4. Under **Scopes**, ensure at least `write:statuses` is checked (required for posting).
5. Click **Submit** to create the application.
6. On the application page, you'll find:
   - **Client key** → set as `MASTODON_CLIENT_KEY`
   - **Client secret** → set as `MASTODON_CLIENT_SECRET`
   - **Your access token** → set as `MASTODON_ACCESS_TOKEN`

If your access token is not shown, click **Create new access token** to generate one with the same scopes.

- **Bluesky**: `internal/bluesky`

#### Creating a Bluesky App Key

To obtain the required `BLUESKY_HANDLE` and `BLUESKY_APPKEY` credentials:

1. Log in to your Bluesky account at [bsky.app](https://bsky.app).
2. Go to **Settings** → **Privacy and Security** → **App Passwords**.
3. Click **Create App Password**.
4. Enter a name for the password (e.g., `rss2socials`).
5. Copy the generated app password — this is your `BLUESKY_APPKEY`.
6. Your `BLUESKY_HANDLE` is your full Bluesky handle (e.g., `yourname.bsky.social`).
- **Threads**: `internal/threads`

#### Creating a Threads Application

To obtain the required `THREADS_CLIENT_ID`, `THREADS_CLIENT_SECRET`, and `THREADS_ACCESS_TOKEN` credentials:

1. Go to the [Meta for Developers](https://developers.facebook.com/) portal and create a new app.
2. Add the **Threads API** product to your app.
3. Under **Settings** → **Basic**, copy the **App ID** → set as `THREADS_CLIENT_ID`.
4. Copy the **App Secret** → set as `THREADS_CLIENT_SECRET`.
5. Set a **Redirect URI** (e.g., `https://yourapp.com/callback`) → set as `THREADS_REDIRECT_URI`.
6. Configure the desired scopes (at minimum `threads_basic` and `threads_content_publish`).
7. Use the OAuth flow to obtain an access token:
   - Visit the authorization URL with your client ID and redirect URI.
   - After the user authorizes, exchange the authorization code for a short-lived token.
   - Exchange the short-lived token for a long-lived token → set as `THREADS_ACCESS_TOKEN`.
8. Optionally, set `THREADS_USER_ID` to your Threads user ID (retrieved via the `/me` endpoint after authentication).

See the [Threads API documentation](https://developers.facebook.com/docs/threads) for more details.

### Database Management (internal/db/db.go)
- Manages an SQLite database to store and check previously posted items.
- Database path defaults to `./tooted_posts.db`; override with the `DB_PATH` environment variable.
- When running via Docker, mount the database file as a volume: `./data/db/sqlite.db:/data/db/sqlite.db` and set `DB_PATH=/data/db/sqlite.db`.
- Tracks `startup_time` per post to support the PostNewEntriesOnly dedup behavior.
- On first startup with `POST_NEW_ENTRIES_ONLY=true`, existing feed entries are stored in the DB but not posted to any social site. Only new entries appearing in subsequent feed checks are posted.

## update golang version
- `make update-golang-version`

