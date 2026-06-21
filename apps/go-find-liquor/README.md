# go-find-liquor (GFL)

![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/toozej/go-find-liquor)
[![Go Report Card](https://goreportcard.com/badge/github.com/toozej/go-find-liquor)](https://goreportcard.com/report/github.com/toozej/go-find-liquor)
![GitHub Actions Workflow Status](https://img.shields.io/github/actions/workflow/status/toozej/go-find-liquor/cicd.yaml)
![Docker Pulls](https://img.shields.io/docker/pulls/toozej/go-find-liquor)
![GitHub Downloads (all assets, all releases)](https://img.shields.io/github/downloads/toozej/go-find-liquor/total)

Oregon Liquor Search Notification Service using [the OLCC Liquor Search website](http://www.oregonliquorsearch.com/), Go, and the [nikoksr/notify library](https://github.com/nikoksr/notify).

## Features

- Search [Oregon Liquor Search](http://www.oregonliquorsearch.com/) for specific liquor items
- Search by product name or item code
- **Multi-user support**: Configure multiple users with individual preferences
- **Notification condensing**: Combine multiple findings into single notifications
- Configurable search radius based on zip code
- Automatic age verification handling
- Random user agent rotation to avoid detection
- Random delays between searches to simulate human behavior
- Multiple notification methods:
  - Gotify (direct API integration)
  - Slack
  - Telegram
  - Discord
  - Pushover
  - Pushbullet
- Configurable search interval
- One-time or continuous search mode
- Backward compatibility with existing single-user configurations

## Usage with Docker

Dockerized GFL uses a final image based on [Distroless Debian static](https://github.com/GoogleContainerTools/distroless) since it includes minimal dependencies but most crucially ca-certificates (which are needed to POST notifications to HTTPS endpoints).

```bash
cp config.example.yaml config.yaml
# edit config.yaml
make build run
```

### Docker Compose

```bash
cp config.example.yaml config.yaml
# edit config.yaml
make up
```

## Installation

```bash
# Clone the repository
git clone https://github.com/toozej/go-find-liquor.git
cd go-find-liquor

# Build the application
make local-build
```

## Multi-User Setup

GFL supports multiple users with individual search preferences. This is useful for:

- **Shared installations**: Multiple people using the same GFL instance
- **Different search areas**: Users in different zip codes
- **Varied notification preferences**: Some users want individual notifications, others prefer condensed
- **Different liquor interests**: Each user can search for their preferred items

### Setting Up Multiple Users

1. **Copy the example configuration**:
   ```bash
   cp config.example.yaml config.yaml
   ```

2. **Edit the configuration** to add your users:
   ```yaml
   # Global settings (shared by all users)
   interval: 6h
   verbose: true
   
   # Individual user configurations
   users:
     - name: "alice"
       items: ["Blanton's", "Eagle Rare"]
       zipcode: "97201"
       distance: 15
       notifications:
         - type: gotify
           condense: false
           credential:
             token: "ALICE_TOKEN"
     
     - name: "bob"
       items: ["Buffalo Trace", "Weller"]
       zipcode: "97210"
       distance: 10
       notifications:
         - type: slack
           condense: true
           credential:
             token: "BOB_TOKEN"
             channel_id: "BOB_CHANNEL"
   ```

3. **Run GFL**: The application will automatically manage searches for all configured users

### Migration from Single-User

If you have an existing single-user configuration, GFL will automatically migrate it:

- Your existing settings become a user named "default"
- All functionality is preserved
- You can add additional users to the same config file
- No manual migration required

## Configuration

GFL supports both single-user and multi-user configurations. You can configure it using a YAML configuration file, environment variables, or command-line flags.

### Multi-User Configuration

GFL now supports multiple users with individual search preferences while sharing global settings like search intervals and logging configuration.

#### Configuration Structure

- **Global Settings**: Apply to all users (interval, verbose logging, user agent)
- **User-Specific Settings**: Each user has their own items, location, and notification preferences
- **Notification Condensing**: Each notification method can be configured to send individual notifications or condense multiple findings into a single message

#### Configuration File

Create a `config.yaml` file in the same directory as the executable:

```bash
cp config.example.yaml config.yaml
# Edit config.yaml to add your users and notification settings
```

#### Multi-User Example

```yaml
# Global settings
interval: 6h
verbose: true

# Multiple users with different preferences
users:
  - name: "alice"
    items:
      - "Blanton's"
      - "W.L. Weller Special Reserve"
    zipcode: "97201"
    distance: 15
    notifications:
      - type: gotify
        endpoint: "https://gotify.example.com"
        condense: false  # Individual notifications
        credential:
          token: "ALICE_GOTIFY_TOKEN"
  
  - name: "bob"
    items:
      - "Eagle Rare"
      - "Buffalo Trace"
    zipcode: "97210"
    distance: 10
    notifications:
      - type: slack
        condense: true  # Condensed notifications
        credential:
          token: "BOB_SLACK_TOKEN"
          channel_id: "BOB_SLACK_CHANNEL"
```

### Single-User Configuration (Legacy Support)

GFL maintains backward compatibility with existing single-user configurations. If you have an existing config, it will be automatically migrated to the multi-user format with a user named "default".

### Environment Variables

Environment variables are supported for backward compatibility with the `GFL_` prefix:

```bash
export GFL_ITEMS="Blanton's,Eagle Rare,Buffalo Trace"
export GFL_ZIPCODE="97201"
export GFL_DISTANCE="15"
export GFL_INTERVAL="6h"
```

**Note**: Environment variables will create a single user configuration and are primarily for backward compatibility.

### Command-Line Flags

Basic options can be set using command-line flags:

```bash
# Run in debug mode
./out/go-find-liquor -d

# Use a specific config file (overrides default config.yaml)
./out/go-find-liquor -c /path/to/config.yaml

# Run search once and exit
./out/go-find-liquor -o
```

### Notification Condensing

Each notification method supports a `condense` option:

- **`condense: false`** (default): Send separate notifications for each liquor item found
- **`condense: true`**: Combine all liquor findings from a single search run into one notification

Example condensed notification:
```
🥃 Liquor Found (3 items):
• Blanton's - Store A (5 miles)
• Eagle Rare - Store B (8 miles)  
• Buffalo Trace - Store C (12 miles)
```

### Configuration Migration

When upgrading from a single-user configuration, GFL will automatically:

1. Detect the legacy format
2. Create a new user named "default" with your existing settings
3. Preserve all functionality while enabling multi-user capabilities
4. Log the migration process for your awareness

You can then manually edit your config to add additional users or rename the default user.

## Usage Examples

### Run a single search and exit

```bash
make local-run

# or alternatively without using the provided Makefile
./out/go-find-liquor --once
```

### Run continuously with the default interval

```bash
./out/go-find-liquor
```

### Run with a specific config file

```bash
./out/go-find-liquor --config /path/to/config.yaml
```

### View version information

```bash
./out/go-find-liquor version
```

### Generate man pages

```bash
./out/go-find-liquor man --dir /usr/local/share/man/man1
```

## Notification Types

GFL supports multiple notification methods. Each notification method supports the `condense` option to control whether multiple findings are combined into a single message.

### Gotify

```yaml
notifications:
  - type: gotify
    endpoint: "https://gotify.example.com"
    condense: false  # Send individual notifications (default)
    credential:
      token: "YOUR_GOTIFY_TOKEN"
```

### Slack

```yaml
notifications:
  - type: slack
    condense: true  # Combine multiple findings into one message
    credential:
      token: "YOUR_SLACK_TOKEN"
      channel_id: "https://exampleorg.slack.com/archives/XXXXXXXXXXXXXXXXXXXXXXXX"
```

### Telegram

```yaml
notifications:
  - type: telegram
    condense: false
    credential:
      token: "YOUR_TELEGRAM_BOT_TOKEN"
      chat_id: "YOUR_CHAT_ID"
```

### Discord

```yaml
notifications:
  - type: discord
    condense: true
    credential:
      webhook_url: "https://discord.com/api/webhooks/000000000000000000/XXXXXXXXXXXXXXXXXXXXX"
```

### Pushover

```yaml
notifications:
  - type: pushover
    condense: false
    credential:
      token: "YOUR_PUSHOVER_TOKEN"
      receipient_id: "XXXXXXXXXXXXX"
```

### Pushbullet

```yaml
notifications:
  - type: pushbullet
    condense: true
    credential:
      token: "YOUR_PUSHBULLET_TOKEN"
      device_nickname: "XXXXXXXXXXXXX"
```

### Notification Behavior

- **Individual Notifications** (`condense: false`): Each liquor item found generates a separate notification
- **Condensed Notifications** (`condense: true`): All liquor items found in a single search run are combined into one notification with a list of all items

## Background

go-find-liquor, or GFL for short, was built since it is increasingly difficult to find some liquors at Oregon liquor stores due to short supply, mis-management, antiquated technology, etc. GFL was born to make it easier to find just the right bottle. Also, fun fact, GFL's alternative name is "good-fucking-luck", as in good luck finding those rare bottles ;).

## changes required to update golang version
- `make update-golang-version`
