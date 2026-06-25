# User Guide

Complete guide to using Podgrab for podcast management.

## Getting Started

### First Time Setup

1. **Access Podgrab**

   - Open browser to `http://localhost:8080`
   - If password protected, enter credentials:
     - Username: `podgrab`
     - Password: `<your configured password>`

1. **Configure Settings**

   - Click Settings (gear icon)
   - Configure initial preferences (see [Configuration Guide](configuration.md))
   - Recommended first-time settings:
     - Download on Add: ✅
     - Initial Download Count: 5
     - Auto Download: ✅

1. **Add Your First Podcast**

   - Click "+ Add Podcast"
   - Choose method:
     - **Direct URL**: Paste RSS feed URL
     - **Search**: Find podcasts via iTunes or Podcast Index
     - **Import OPML**: Bulk import from another app

## Adding Podcasts

### Method 1: Direct RSS URL

**When to use:** You have the RSS feed URL

```
1. Click "+ Add Podcast"
2. Paste RSS feed URL
   Example: https://feeds.simplecast.com/54nAGcIl
3. Click "Add Podcast"
4. Wait for processing (1-5 seconds)
5. Podcast appears in home view
```

**Finding RSS URLs:**

- Podcast website (usually "Subscribe" or "RSS" link)
- Podcast player apps (share/copy feed URL)
- Podcast directories (copy feed URL)

### Method 2: Search

**When to use:** You don't have the feed URL

```
1. Click "+ Add Podcast"
2. Select search source:
   - iTunes: Apple's podcast directory
   - PodcastIndex: Open podcast directory
3. Enter search query
4. Browse results
5. Click "Add" on desired podcast
```

**Search Tips:**

- Use podcast name or host name
- Try different search sources for better results
- "Already Added" indicator shows existing podcasts

### Method 3: Import OPML

**When to use:** Migrating from another podcast app

```
1. Export OPML from current app:
   - Most apps: Settings → Export OPML
2. In Podgrab: Settings → Import OPML
3. Select exported OPML file
4. Click Upload
5. Wait for import (may take several minutes)
6. All podcasts appear in home view
```

**OPML Export from Popular Apps:**

- **Apple Podcasts**: File → Library → Export Library
- **Pocket Casts**: Settings → Import/Export → Export
- **Overcast**: Settings → OPML Export
- **AntennaPod**: Settings → Storage → Export

## Managing Podcasts

### Podcast List View

**Home Screen** shows all podcasts with:

- Cover artwork
- Podcast title
- Episode counts:
  - Downloaded episodes (green)
  - Queued for download (yellow)
  - Total episodes
- Storage usage

**Sorting Options:**

- Date Added (newest/oldest)
- Alphabetical (A-Z / Z-A)
- Last Episode (newest/oldest)

### Podcast Detail View

Click podcast to see:

- Podcast description
- All episodes (paginated)
- Episode controls
- Podcast actions

**Episode List:**

- Title and description
- Publication date
- Duration
- Download status
- Action buttons

### Podcast Actions

#### Download All Episodes

```
1. Open podcast detail page
2. Click "Download All"
3. Episodes queue for download
4. Monitor progress in episode list
```

**Note:** Respects MaxDownloadConcurrency setting (default: 5 concurrent)

#### Pause Podcast

**When to use:** Temporarily stop downloading new episodes

```
1. Open podcast detail page
2. Click "Pause"
3. Podcast marked as paused
```

**Effect:**

- No new episodes downloaded automatically
- Can still manually download episodes
- Can unpause anytime

#### Delete Podcast

**Options:**

**Delete Everything:**

```
1. Click "Delete Podcast"
2. Confirm deletion
```

**Removes:**

- Podcast from database
- All episode database records
- All downloaded episode files

**Delete Podcast Only:**

```
1. Click "Delete Podcast Only"
2. Confirm deletion
```

**Removes:**

- Podcast from database **Keeps:**
- Downloaded episode files (orphaned)

**Delete Episodes Only:**

```
1. Click "Delete All Episodes"
2. Confirm deletion
```

**Removes:**

- All downloaded files **Keeps:**
- Podcast in database
- Episode metadata

## Managing Episodes

### Episode Actions

#### Download Episode

**For single episode:**

```
1. Click download icon (⬇️)
2. Download starts immediately
3. Progress indicator shows status
4. Icon changes when complete
```

**For multiple episodes:**

```
1. Use "Download All" on podcast
2. Or individually download each
```

#### Delete Episode

```
1. Click delete icon (🗑️)
2. Confirm deletion
3. File removed from storage
4. Status changes to "Deleted"
5. Can re-download later
```

#### Play Episode

**In Web Player:**

```
1. Click play icon (▶️)
2. Player opens/updates
3. Audio begins playing
```

**Download for External Player:**

```
1. Click episode menu (⋮)
2. Select "Download File"
3. Save to device
4. Open in preferred player
```

#### Mark as Played/Unplayed

```
1. Click checkmark icon
2. Toggles played status
3. Visual indicator updates
4. Used for tracking listening progress
```

#### Bookmark Episode

**Add Bookmark:**

```
1. Click bookmark icon (🔖)
2. Episode added to bookmarks
3. Timestamp recorded
```

**View Bookmarks:**

```
1. Navigate to Bookmarks view
2. See all bookmarked episodes
3. Sorted by bookmark date
```

**Remove Bookmark:**

```
1. Click bookmark icon again
2. Episode removed from bookmarks
```

### Filtering Episodes

**All Episodes View** (`/episodes`) offers filters:

**By Podcast:**

```
1. Select podcast from dropdown
2. Shows only episodes from that podcast
```

**By Tag:**

```
1. Select tag from dropdown
2. Shows episodes from all tagged podcasts
```

**By Status:**

```
- Only Downloaded: Shows downloaded episodes only
- Only Bookmarked: Shows bookmarked episodes only
```

**Sorting:**

- Release Date (newest/oldest)
- Duration (shortest/longest)

**Pagination:**

- Adjust items per page (10/20/50/100)
- Navigate pages

## Using Tags

### Create Tag

```
1. Navigate to Tags page
2. Click "Create Tag"
3. Enter:
   - Label: "Technology"
   - Description: "Tech podcasts"
4. Save tag
```

### Assign Tags to Podcasts

**From Podcast Page:**

```
1. Open podcast detail page
2. Click "Add Tag"
3. Select tag(s) from list
4. Tags appear under podcast info
```

**Bulk Tagging:**

```
1. Tag multiple podcasts with same tag
2. Use tag filters to view all together
```

### Using Tags

**Organization Examples:**

- **By Topic**: Technology, News, Comedy, Education
- **By Priority**: Must Listen, Casual, Archive
- **By Schedule**: Daily, Weekly, Monthly
- **By Mood**: Relaxing, Educational, Entertaining

**Filter by Tag:**

```
1. Episodes page → Select tag
2. Player page → Queue by tag
3. RSS feed → Generate tag-specific feed
```

### Remove Tag from Podcast

```
1. Open podcast detail page
2. Click X on tag
3. Tag removed from podcast
4. Tag still exists for other uses
```

### Delete Tag

```
1. Tags page → Find tag
2. Click delete
3. Confirm deletion
4. Removed from all podcasts
```

## Web Player

### Basic Playback

**Start Playback:**

```
1. Click play on any episode
2. Player page opens (or updates)
3. Audio begins playing
```

**Player Controls:**

- **Play/Pause**: ▶️ ⏸️
- **Previous/Next**: ⏮️ ⏭️
- **Seek**: Drag progress bar
- **Volume**: Adjust slider
- **Speed**: 0.5x - 2.0x (if available)

### Queue Management

**Add to Queue:**

**Single Episode:**

```
1. Click play icon
2. Episode added to queue
```

**Multiple Episodes:**

```
Method 1: Select episodes
1. Check episodes in list
2. Click "Add to Queue"

Method 2: Play all from podcast
1. Podcast page → "Play All"
2. All episodes queue
```

**Reorder Queue:**

```
1. Drag and drop episodes
2. Queue updates immediately
```

**Clear Queue:**

```
1. Click "Clear Queue"
2. Removes all episodes
3. Stops playback
```

### Multi-Tab Sync

**Scenario:** Player in one tab, browsing in another

**Setup:**

```
Tab 1: Open player page (/player)
Tab 2: Browse podcasts/episodes
```

**Behavior:**

- Queue episodes from Tab 2
- Automatically appear in Tab 1
- Playback state syncs
- Both tabs show current episode

**Requirements:**

- WebSocket connection active
- Both tabs from same browser
- No browser extensions blocking WebSocket

### Player Page Options

**Play All from Podcast:**

```
URL: /player?podcastId=<podcast-id>
```

**Play Tagged Episodes:**

```
URL: /player?tagIds[]=<tag-id1>&tagIds[]=<tag-id2>
```

**Play Specific Episodes:**

```
URL: /player?itemIds[]=<episode-id1>&itemIds[]=<episode-id2>
```

**Play Latest:**

```
URL: /player (no parameters)
Shows 20 most recent downloaded episodes
```

## OPML Management

### Export OPML

**Standard Export (Original URLs):**

```
1. Settings → Export OPML
2. Download podgrab-export.opml
3. Import in other podcast app
```

**Podgrab Links Export:**

```
1. Settings → Export OPML
2. Enable "Use Podgrab Links"
3. Download OPML
4. Feed URLs point to your Podgrab instance
```

**Use Cases:**

- **Backup**: Save subscription list
- **Migration**: Move to another app
- **Sharing**: Share custom feeds with others
- **Multi-Device**: Sync across devices

### Import OPML

```
1. Settings → Import OPML
2. Select OPML file
3. Click Upload
4. Wait for import
5. Podcasts appear in list
```

**Behavior:**

- Duplicate detection (no duplicates added)
- Parallel processing (faster import)
- Auto-download triggered (if enabled)

## RSS Feed Generation

Podgrab generates RSS feeds for sharing and external consumption.

### Podcast Feed

**URL:** `/podcasts/:id/rss`

**Usage:**

```
1. Navigate to podcast detail page
2. Copy RSS feed URL
3. Subscribe in any podcast app
```

**Features:**

- Contains all podcast episodes
- Media files served from Podgrab
- Updates when you download new episodes

### Tag Feed

**URL:** `/tags/:id/rss`

**Usage:**

```
1. Navigate to tag page
2. Copy RSS feed URL
3. Subscribe in any podcast app
```

**Features:**

- Aggregates episodes from all tagged podcasts
- Custom playlist functionality
- Updates automatically

### Global Feed

**URL:** `/rss`

**Features:**

- All episodes from all podcasts
- Master feed of your entire library
- Useful for "play all" scenarios

### Feed Authentication

If password protection enabled:

- Feeds require HTTP Basic Auth
- Username: `podgrab`
- Password: `<your password>`

**Example:**

```
https://podgrab:password@podgrab.example.com/podcasts/abc123/rss
```

## Settings

See [Configuration Guide](configuration.md) for detailed settings documentation.

### Quick Settings Overview

**Download Settings:**

- **Download on Add**: Auto-download when adding podcast
- **Initial Download Count**: How many episodes to download initially
- **Auto Download**: Download new episodes automatically
- **Max Concurrency**: Concurrent downloads (default: 5)

**File Naming:**

- **Append Date**: Add date to filename
- **Append Episode Number**: Add episode number to filename

**UI Settings:**

- **Dark Mode**: Dark theme
- **Base URL**: Custom URL for RSS feeds

**Advanced:**

- **Download Episode Images**: Save episode artwork locally
- **Generate NFO Files**: Create metadata files
- **Don't Re-download Deleted**: Skip manually deleted episodes
- **User Agent**: Custom user agent for downloads

## Keyboard Shortcuts

**Player:**

- `Space`: Play/Pause
- `←/→`: Seek backward/forward
- `↑/↓`: Volume up/down
- `M`: Mute/Unmute
- `F`: Full screen (if supported)

**Navigation:**

- `H`: Home
- `A`: Add Podcast
- `E`: All Episodes
- `T`: Tags
- `S`: Settings
- `?`: Help (if available)

## Mobile Usage

Podgrab is responsive and works on mobile browsers.

### Mobile Tips

**Install as Web App (PWA):**

```
iOS Safari:
1. Share → Add to Home Screen
2. Launch from home screen

Android Chrome:
1. Menu → Add to Home Screen
2. Launch from home screen
```

**Offline Listening:**

- Downloaded episodes remain accessible
- Queue episodes before going offline
- Player works without internet

**Mobile Player:**

- Touch controls work
- Background audio (browser dependent)
- Lock screen controls (browser dependent)

## Advanced Features

### Custom RSS Feed with Tags

**Create Custom Playlist:**

```
1. Create tag: "Morning Commute"
2. Tag favorite podcasts
3. Generate RSS: /tags/<tag-id>/rss
4. Subscribe in car's podcast app
```

### Podgrab as Podcast Server

**Share your library:**

```
1. Enable password protection
2. Configure reverse proxy with HTTPS
3. Share podcast/tag RSS feeds
4. Friends subscribe to your curated feeds
```

**Use Cases:**

- Family podcast sharing
- Curated recommendations
- Private podcast distribution

### Automated Workflows

**Example: Daily news digest**

```
1. Tag news podcasts with "Daily News"
2. Generate tag RSS feed
3. Subscribe in automation tool (IFTTT, Zapier)
4. Trigger actions based on new episodes
```

## Troubleshooting

### Common Issues

**Podcast won't add:**

- Verify RSS URL is correct
- Check feed is accessible (try in browser)
- Look for error messages
- Check logs for details

**Episodes not downloading:**

- Check storage space
- Verify network connectivity
- Check MaxDownloadConcurrency setting
- Look for background job execution
- Review logs for errors

**Player not working:**

- Check browser compatibility
- Verify file downloaded successfully
- Try different browser
- Check console for errors

**WebSocket disconnected:**

- Check reverse proxy configuration
- Verify WebSocket upgrade headers
- Test with different browser
- Check firewall rules

**Dark mode not saving:**

- Clear browser cache
- Check browser cookies enabled
- Verify settings POST successful

### Getting Help

1. **Check documentation**: Search docs for your issue
1. **GitHub Issues**: Search existing issues
1. **Create Issue**: If not found, create new issue with details
1. **Community**: Discord/Reddit (if available)

## Best Practices

### Organization

- **Use descriptive tags** for easy filtering
- **Regular cleanup** of old episodes
- **Bookmark** episodes to listen later
- **Pause** podcasts you're not actively listening to

### Storage Management

- **Monitor disk usage** in settings
- **Delete old episodes** you won't re-listen
- **Adjust download settings** based on listening habits
- **Use selective downloads** instead of "download all"

### Performance

- **Limit concurrent downloads** on slow systems
- **Stagger new podcast additions** to avoid overwhelming system
- **Use SSD storage** for database (if possible)
- **Regular backups** of database

### Privacy & Security

- **Use strong passwords** if exposing to internet
- **Keep Podgrab updated** for security patches
- **Use HTTPS** in production
- **Backup regularly** to prevent data loss

## Tips & Tricks

### Power User Tips

**Quick add from bookmarklet:**

```javascript
javascript:(function(){window.location='http://localhost:8080/add?url='+encodeURIComponent(window.location.href);})()
```

**RSS autodiscovery:**

- Some podcast websites have RSS in HTML `<link>` tags
- Browser extensions can detect and offer to add

**Batch processing:**

- Import OPML with many podcasts
- Let initial downloads complete overnight
- Organize with tags the next day

**Custom playlists:**

- Use tags creatively (mood, activity, length)
- Generate RSS feeds for each playlist
- Subscribe to own feeds in external apps

## Glossary

**OPML**: Outline Processor Markup Language - subscription list format **RSS**:
Really Simple Syndication - podcast feed format **Episode**: Individual podcast
audio file **Feed**: XML file containing podcast information **WebSocket**:
Real-time communication protocol **Tag**: Label for organizing podcasts
**Bookmark**: Saved episode for later listening **Queue**: Playlist of episodes
to play

## Related Documentation

- [Configuration Guide](configuration.md) - Detailed settings
- [REST API](../api/rest-api.md) - API reference
- [WebSocket API](../api/websocket.md) - Real-time features
- [Docker Deployment](../deployment/docker.md) - Installation

______________________________________________________________________

**Enjoy your podcast management with Podgrab!** 🎧
