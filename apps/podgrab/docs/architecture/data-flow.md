# Data Flow

This document details how data flows through Podgrab for various operations,
including request/response patterns, background job flows, and real-time update
mechanisms.

## Core Data Flows

### 1. Add Podcast Flow

```mermaid
sequenceDiagram
    actor User
    participant UI as Web UI
    participant Controller as Podcast Controller
    participant Service as Podcast Service
    participant iTunes as iTunes Service
    participant RSS as RSS Feed
    participant DB as Database
    participant FS as File System
    participant WS as WebSocket

    User->>UI: Enter Podcast URL
    UI->>Controller: POST /podcasts {url}

    alt iTunes URL
        Controller->>iTunes: Lookup Podcast
        iTunes-->>Controller: Feed URL
    end

    Controller->>Service: AddPodcast(url)
    Service->>RSS: HTTP GET Feed XML
    RSS-->>Service: RSS XML Response

    Service->>Service: Parse RSS Feed
    Service->>Service: Extract Podcast Metadata
    Service->>Service: Extract Episodes

    Service->>DB: Check if Podcast Exists
    alt Podcast Exists
        Service->>DB: Update Podcast Metadata
    else New Podcast
        Service->>DB: INSERT Podcast
    end

    loop For Each Episode
        Service->>DB: Check if Episode Exists
        alt Episode Exists
            Service->>DB: UPDATE Episode
        else New Episode
            Service->>DB: INSERT PodcastItem
        end
    end

    alt Download on Add (Setting)
        Service->>Service: Queue Downloads
        Service->>WS: Broadcast "Queued for Download"
    end

    Service-->>Controller: Podcast + Episodes Created
    Controller-->>UI: 200 OK + Podcast Data
    UI-->>User: Show Podcast Page
```

### 2. Episode Download Flow

```mermaid
sequenceDiagram
    participant Job as Background Job
    participant Service as File Service
    participant DB as Database
    participant HTTP as HTTP Client
    participant FS as File System
    participant WS as WebSocket
    participant Client as Web Browser

    Job->>DB: Get Episodes with Status=NotDownloaded
    DB-->>Job: Episode List

    loop For Each Episode (up to MaxConcurrency)
        Job->>Service: Download(episode)

        Service->>FS: Check if File Exists
        alt File Exists
            Service->>DB: UPDATE Status=Downloaded
            Service->>Service: Skip Download
        else File Missing
            Service->>DB: UPDATE Status=Downloading
            Service->>WS: Broadcast "Downloading"
            WS-->>Client: Update Progress UI

            Service->>FS: Create Podcast Directory
            Service->>HTTP: Stream Download
            HTTP-->>Service: File Chunks

            loop Write Chunks
                Service->>FS: Write Chunk
            end

            Service->>FS: Verify File Integrity
            alt Download Success
                Service->>DB: UPDATE Status=Downloaded, Path, Size
                Service->>WS: Broadcast "Complete"
                WS-->>Client: Update UI (Downloaded)
            else Download Failed
                Service->>FS: Delete Partial File
                Service->>DB: UPDATE Status=NotDownloaded
                Service->>WS: Broadcast "Failed"
                WS-->>Client: Show Error
            end
        end
    end
```

### 3. RSS Feed Refresh Flow

```mermaid
flowchart TD
    Start([Cron Trigger]) --> Lock{Acquire Job Lock?}

    Lock -->|Success| GetPodcasts[Get All Active Podcasts]
    Lock -->|Failed| End1([Skip - Already Running])

    GetPodcasts --> Loop{More Podcasts?}

    Loop -->|Yes| CheckPaused{Podcast Paused?}
    Loop -->|No| ReleaseLock[Release Job Lock]

    CheckPaused -->|Yes| Loop
    CheckPaused -->|No| FetchRSS[Fetch RSS Feed]

    FetchRSS --> ParseSuccess{Parse Success?}

    ParseSuccess -->|Yes| CompareEpisodes[Compare with DB Episodes]
    ParseSuccess -->|No| LogError[Log Error]

    CompareEpisodes --> NewEpisodes{New Episodes?}

    NewEpisodes -->|Yes| AddEpisodes[Add New Episodes to DB]
    NewEpisodes -->|No| UpdateMeta[Update Podcast Metadata]

    AddEpisodes --> AutoDownload{Auto-Download Enabled?}

    AutoDownload -->|Yes| QueueDownloads[Queue Episodes for Download]
    AutoDownload -->|No| NotifyNew[Notify via WebSocket]

    QueueDownloads --> NotifyNew
    NotifyNew --> UpdateMeta

    UpdateMeta --> Loop
    LogError --> Loop

    ReleaseLock --> End2([Complete])

    style Start fill:#90EE90
    style End1 fill:#FFD700
    style End2 fill:#90EE90
    style FetchRSS fill:#87CEEB
    style QueueDownloads fill:#FFA07A
```

### 4. WebSocket Real-time Updates

```mermaid
sequenceDiagram
    participant Client1 as Browser 1
    participant Client2 as Browser 2
    participant WSHandler as WebSocket Handler
    participant Hub as Message Hub
    participant Service as Service Layer
    participant DB as Database

    Client1->>WSHandler: WebSocket Connect /ws
    Client2->>WSHandler: WebSocket Connect /ws

    WSHandler->>Hub: Register Client 1
    WSHandler->>Hub: Register Client 2

    Note over Hub: Hub runs in background goroutine

    Service->>DB: Update Episode Status
    DB-->>Service: Success

    Service->>Hub: Send Message {type, data}

    Hub->>Hub: Iterate Connected Clients

    par Broadcast to All
        Hub->>Client1: WebSocket Message
        Hub->>Client2: WebSocket Message
    end

    Client1->>Client1: Update UI
    Client2->>Client2: Update UI

    Note over Client1,Client2: Real-time UI update without polling
```

## Data Transformation Flows

### RSS to Database

```mermaid
flowchart LR
    subgraph "RSS Feed"
        RSS[XML RSS Feed]
    end

    subgraph "Parsing"
        XML[xml.Unmarshal]
        Model[PodcastData Model]
    end

    subgraph "Transformation"
        Extract[Extract Metadata]
        Sanitize[Sanitize HTML]
        Normalize[Normalize Dates]
    end

    subgraph "Database"
        Podcast[(Podcast Record)]
        Episodes[(PodcastItem Records)]
    end

    RSS --> XML
    XML --> Model
    Model --> Extract
    Extract --> Sanitize
    Sanitize --> Normalize
    Normalize --> Podcast
    Normalize --> Episodes

    style RSS fill:#E8F4F8
    style Model fill:#FFF9E6
    style Podcast fill:#90EE90
    style Episodes fill:#90EE90
```

**Transformation Steps**:

1. **XML Parsing**: `encoding/xml` → `model.PodcastData`
1. **HTML Sanitization**: `bluemonday` removes unsafe HTML
1. **Text Cleanup**: `html-strip-tags-go` removes all HTML tags
1. **Date Parsing**: RFC822/RFC3339 → `time.Time`
1. **URL Validation**: Ensure valid episode file URLs
1. **GUID Extraction**: Use GUID for episode uniqueness

### File Download to Storage

```mermaid
flowchart TD
    URL[Episode File URL] --> Request[HTTP GET Request]
    Request --> Stream[Streaming Response]

    Stream --> Buffer[Buffer Chunks<br/>4KB-32KB]
    Buffer --> Sanitize[Sanitize Filename]

    Sanitize --> Prefix{Add Prefix?}
    Prefix -->|Date| AddDate[Add Date Prefix]
    Prefix -->|Episode#| AddNum[Add Episode Number]
    Prefix -->|Both| AddBoth[Add Both]
    Prefix -->|None| NoPrefix[Use Title Only]

    AddDate --> CreatePath[Create Full Path]
    AddNum --> CreatePath
    AddBoth --> CreatePath
    NoPrefix --> CreatePath

    CreatePath --> CreateDir[Create Directory<br/>if Missing]
    CreateDir --> Write[Write to File System]
    Write --> Verify[Verify File Size]

    Verify --> Update[(Update Database<br/>Path + Size + Status)]

    style URL fill:#E8F4F8
    style Stream fill:#87CEEB
    style Write fill:#FFA07A
    style Update fill:#90EE90
```

## Request/Response Patterns

### REST API Patterns

#### List Resources (GET)

```mermaid
sequenceDiagram
    Client->>Router: GET /podcasts?sort=name&order=asc
    Router->>Middleware: Check Auth
    Middleware->>Controller: GetAllPodcasts()
    Controller->>Service: GetAllPodcasts(sorting)
    Service->>DB: SELECT with ORDER BY
    DB-->>Service: Podcast Array
    Service->>Service: Calculate Statistics
    Service-->>Controller: Podcasts + Stats
    Controller-->>Client: 200 OK + JSON Array

    Note over Client,Controller: Standard REST pattern with sorting
```

#### Get Single Resource (GET)

```mermaid
sequenceDiagram
    Client->>Router: GET /podcasts/:id
    Router->>Middleware: Check Auth
    Middleware->>Controller: GetPodcastById(id)
    Controller->>Service: GetPodcastById(id)
    Service->>DB: SELECT WHERE id=?
    alt Podcast Found
        DB-->>Service: Podcast Record
        Service->>DB: Preload Episodes & Tags
        Service-->>Controller: Complete Podcast
        Controller-->>Client: 200 OK + JSON
    else Not Found
        DB-->>Service: nil
        Service-->>Controller: nil
        Controller-->>Client: 404 Not Found
    end
```

#### Create Resource (POST)

```mermaid
sequenceDiagram
    Client->>Router: POST /podcasts {url}
    Router->>Middleware: Check Auth & Validate
    Middleware->>Controller: AddPodcast()
    Controller->>Service: AddPodcast(url)
    Service->>Service: Fetch & Parse RSS
    Service->>DB: BEGIN TRANSACTION
    Service->>DB: INSERT Podcast
    Service->>DB: INSERT Episodes (Batch)
    Service->>DB: COMMIT
    Service-->>Controller: Created Podcast
    Controller-->>Client: 201 Created + Location Header
```

#### Update Resource (PATCH)

```mermaid
sequenceDiagram
    Client->>Router: PATCH /podcastitems/:id {isPlayed: true}
    Router->>Middleware: Check Auth & Validate
    Middleware->>Controller: PatchPodcastItemById(id)
    Controller->>Service: GetPodcastItemById(id)
    Service->>DB: SELECT WHERE id=?
    alt Found
        DB-->>Service: PodcastItem
        Service->>Service: Update Fields
        Service->>DB: UPDATE WHERE id=?
        Service-->>Controller: Updated Item
        Controller-->>Client: 200 OK + JSON
    else Not Found
        Controller-->>Client: 404 Not Found
    end
```

#### Delete Resource (DELETE)

```mermaid
sequenceDiagram
    Client->>Router: DELETE /podcasts/:id
    Router->>Middleware: Check Auth
    Middleware->>Controller: DeletePodcastById(id)
    Controller->>Service: DeletePodcast(id)
    Service->>DB: Get Podcast + Episodes
    Service->>FS: Delete Episode Files
    Service->>DB: BEGIN TRANSACTION
    Service->>DB: DELETE Episodes
    Service->>DB: DELETE Podcast Tags
    Service->>DB: DELETE Podcast
    Service->>DB: COMMIT
    Service-->>Controller: Success
    Controller-->>Client: 204 No Content
```

### HTML Page Rendering

```mermaid
sequenceDiagram
    Client->>Router: GET /
    Router->>Middleware: Check Auth & Setup Settings
    Middleware->>Controller: HomePage()
    Controller->>Service: GetAllPodcasts(sort)
    Service->>DB: SELECT Podcasts with Stats
    DB-->>Service: Podcasts + Episode Counts
    Service-->>Controller: Data
    Controller->>Templates: Render index.html
    Templates->>Templates: Apply Template Functions
    Templates-->>Controller: Rendered HTML
    Controller->>Controller: Inject Settings (Dark Mode, etc)
    Controller-->>Client: 200 OK + HTML Page
```

## Background Job Data Flow

### Job Execution Pattern

```mermaid
stateDiagram-v2
    [*] --> Scheduled: Cron Trigger
    Scheduled --> AcquireLock: Check Lock

    AcquireLock --> Locked: Lock Available
    AcquireLock --> Skipped: Lock Held

    Locked --> FetchData: Get Work Items
    FetchData --> ProcessItems: Iterate
    ProcessItems --> UpdateDB: Write Results
    UpdateDB --> BroadcastWS: Notify Clients
    BroadcastWS --> ReleaseLock: Cleanup
    ReleaseLock --> [*]

    Skipped --> [*]: Another Instance Running

    state ProcessItems {
        [*] --> NextItem
        NextItem --> Execute: Process Item
        Execute --> NextItem: More Items
        Execute --> [*]: Done
    }
```

### Parallel Download Processing

```mermaid
graph TB
    Queue[Download Queue<br/>NotDownloaded Episodes] --> Dispatcher[Download Dispatcher]

    Dispatcher -->|Semaphore Slot 1| Worker1[Worker Goroutine 1]
    Dispatcher -->|Semaphore Slot 2| Worker2[Worker Goroutine 2]
    Dispatcher -->|Semaphore Slot 3| Worker3[Worker Goroutine 3]
    Dispatcher -->|Semaphore Slot N| WorkerN[Worker Goroutine N]

    Worker1 --> Download1[Download Episode]
    Worker2 --> Download2[Download Episode]
    Worker3 --> Download3[Download Episode]
    WorkerN --> DownloadN[Download Episode]

    Download1 --> DB1[(Update DB)]
    Download2 --> DB2[(Update DB)]
    Download3 --> DB3[(Update DB)]
    DownloadN --> DBN[(Update DB)]

    DB1 --> WS[WebSocket Hub]
    DB2 --> WS
    DB3 --> WS
    DBN --> WS

    WS --> Clients[Connected Clients]

    style Queue fill:#FFD700
    style Dispatcher fill:#87CEEB
    style WS fill:#FFA07A
```

## Data Consistency Patterns

### Optimistic Locking (via GORM)

```mermaid
sequenceDiagram
    participant Client1
    participant Client2
    participant DB

    Client1->>DB: SELECT Episode (version=1)
    Client2->>DB: SELECT Episode (version=1)

    Client1->>Client1: Modify Episode
    Client2->>Client2: Modify Episode

    Client1->>DB: UPDATE WHERE id=? AND version=1, SET version=2
    DB-->>Client1: Success (1 row updated)

    Client2->>DB: UPDATE WHERE id=? AND version=1, SET version=2
    DB-->>Client2: Failure (0 rows updated - version changed)

    Client2->>DB: SELECT Episode (version=2)
    Client2->>Client2: Retry with new data
```

### Job Lock Pattern

```mermaid
flowchart TD
    Start([Job Triggered]) --> TryLock{Try Acquire Lock}

    TryLock -->|Success| CheckExpiry{Lock Expired?}
    TryLock -->|Failed - Locked| End1([Skip Execution])

    CheckExpiry -->|No| CreateLock[Create Lock Record]
    CheckExpiry -->|Yes| Unlock[Delete Old Lock]

    Unlock --> CreateLock
    CreateLock --> Execute[Execute Job]
    Execute --> Complete[Job Complete]
    Complete --> DeleteLock[Delete Lock Record]
    DeleteLock --> End2([Success])

    style Start fill:#90EE90
    style Execute fill:#87CEEB
    style End1 fill:#FFD700
    style End2 fill:#90EE90
```

## Error Flow Patterns

### Download Error Recovery

```mermaid
flowchart TD
    Start[Start Download] --> Download{Download Success?}

    Download -->|Success| Verify{File Integrity OK?}
    Download -->|Fail| Cleanup1[Delete Partial File]

    Verify -->|OK| UpdateSuccess[Status=Downloaded]
    Verify -->|Fail| Cleanup2[Delete Corrupted File]

    Cleanup1 --> UpdateFailed[Status=NotDownloaded]
    Cleanup2 --> UpdateFailed

    UpdateFailed --> Log[Log Error]
    Log --> Retry{Retry Count < Max?}

    Retry -->|Yes| Wait[Wait for Next Job Cycle]
    Retry -->|No| MarkPermanentFail[Mark as Failed]

    Wait --> End1([Retry Later])
    MarkPermanentFail --> End2([Give Up])
    UpdateSuccess --> End3([Complete])

    style UpdateSuccess fill:#90EE90
    style UpdateFailed fill:#FF6B6B
    style End3 fill:#90EE90
```

### RSS Parse Error Handling

```mermaid
flowchart TD
    Fetch[Fetch RSS Feed] --> HTTPSuccess{HTTP 200?}

    HTTPSuccess -->|No| LogHTTP[Log HTTP Error]
    HTTPSuccess -->|Yes| Parse[Parse XML]

    Parse --> ParseSuccess{Valid XML?}

    ParseSuccess -->|No| LogParse[Log Parse Error]
    ParseSuccess -->|Yes| ValidateFeed{Valid RSS?}

    ValidateFeed -->|No| LogInvalid[Log Invalid Feed]
    ValidateFeed -->|Yes| ProcessEpisodes[Process Episodes]

    ProcessEpisodes --> EpisodeLoop{More Episodes?}

    EpisodeLoop -->|Yes| ValidateEpisode{Valid Episode?}
    EpisodeLoop -->|No| Success[Complete]

    ValidateEpisode -->|Yes| AddEpisode[Add to DB]
    ValidateEpisode -->|No| SkipEpisode[Skip Episode]

    AddEpisode --> EpisodeLoop
    SkipEpisode --> LogSkip[Log Skipped]
    LogSkip --> EpisodeLoop

    LogHTTP --> SkipPodcast[Skip Podcast Update]
    LogParse --> SkipPodcast
    LogInvalid --> SkipPodcast

    SkipPodcast --> End1([Retry Next Cycle])
    Success --> End2([Complete])

    style Success fill:#90EE90
    style SkipPodcast fill:#FFD700
```

## Related Documentation

- [Overview](overview.md) - System architecture
- [System Design](system-design.md) - Design patterns
- [Database Schema](database-schema.md) - Data model
- [REST API](../api/rest-api.md) - API reference

______________________________________________________________________

**Next Steps**: Review [Database Schema](database-schema.md) for detailed data
model documentation.
