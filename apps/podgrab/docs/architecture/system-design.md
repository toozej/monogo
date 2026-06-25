# System Design

This document details Podgrab's system design, including architectural patterns,
design decisions, and implementation strategies.

## Design Principles

### 1. Simplicity Over Complexity

- Monolithic architecture for easier deployment and maintenance
- Minimal dependencies
- Direct database access without caching layer
- Server-side rendering over complex frontend framework

### 2. Fail-Safe Operations

- Job locking prevents duplicate execution
- Download verification before marking complete
- Existing file detection to prevent re-downloads
- Graceful handling of RSS feed failures

### 3. User Control

- Extensive configuration options
- Manual download control
- Pause/unpause podcasts
- Tag-based organization

## Architectural Patterns

### Layered Architecture

```mermaid
graph TD
    subgraph "Layer 1: Presentation"
        direction LR
        HTTP[HTTP Endpoints]
        Pages[HTML Pages]
        WS[WebSocket]
    end

    subgraph "Layer 2: Application"
        direction LR
        Controllers[Controllers]
        Middleware[Middleware]
    end

    subgraph "Layer 3: Business Logic"
        direction LR
        Services[Services]
        Jobs[Background Jobs]
    end

    subgraph "Layer 4: Data Access"
        direction LR
        DB[Database Layer]
        FS[File System]
    end

    subgraph "Layer 5: Infrastructure"
        direction LR
        SQLite[(SQLite)]
        Storage[File Storage]
    end

    HTTP --> Controllers
    Pages --> Controllers
    WS --> Controllers
    Controllers --> Middleware
    Middleware --> Services
    Services --> Jobs
    Services --> DB
    Services --> FS
    DB --> SQLite
    FS --> Storage

    style "Layer 1: Presentation" fill:#e8f4f8
    style "Layer 2: Application" fill:#f5f5dc
    style "Layer 3: Business Logic" fill:#fff9e6
    style "Layer 4: Data Access" fill:#f0f0f0
    style "Layer 5: Infrastructure" fill:#ffe6e6
```

### Service Pattern

Each service encapsulates a specific domain:

```mermaid
classDiagram
    class PodcastService {
        +AddPodcast(url) Podcast
        +RefreshEpisodes()
        +GetAllPodcasts() []Podcast
        +SearchPodcasts(query) []Podcast
        -parseFeed(xml) PodcastData
        -createEpisodes(items)
    }

    class FileService {
        +Download(url, title, podcast) string
        +CheckMissingFiles()
        +UpdateFileSizes()
        +DownloadMissingImages()
        -getFileName(url, title) string
        -sanitizePath(path) string
    }

    class iTunesService {
        +Search(query) []Result
        +Lookup(id) Podcast
    }

    PodcastService --> FileService : uses
    PodcastService --> iTunesService : uses
```

## Design Patterns Implementation

### 1. Repository Pattern (Simplified)

Database functions act as repositories:

```mermaid
graph LR
    Service[Service Layer] --> DBFunc[Database Functions]
    DBFunc --> GORM[GORM]
    GORM --> SQLite[(SQLite)]

    DBFunc -.implements.- Repo[Repository Interface<br/>Concept]

    style Service fill:#fff9e6
    style DBFunc fill:#f0f0f0
    style Repo fill:#e8f4f8
```

**Implementation**: `db/dbfunctions.go`

- `GetPodcastById(id, podcast)`
- `GetAllPodcasts(podcasts, sorting)`
- `AddOrUpdatePodcast(podcast)`
- `DeletePodcast(id)`

### 2. Template Method Pattern

Download workflow uses template method:

```mermaid
graph TD
    Start[Start Download] --> Validate[Validate URL]
    Validate --> CheckExist{File Exists?}
    CheckExist -->|Yes| UpdateMeta[Update Metadata]
    CheckExist -->|No| CreateDir[Create Directory]
    CreateDir --> Download[Download File]
    Download --> Verify[Verify Download]
    Verify --> UpdateDB[Update Database]
    UpdateDB --> Broadcast[Broadcast Progress]
    UpdateMeta --> Broadcast
    Broadcast --> End[Complete]

    style Start fill:#90EE90
    style End fill:#90EE90
    style Download fill:#FFD700
    style Verify fill:#FFD700
```

**Implementation**: `service/fileService.go:Download()`

### 3. Observer Pattern (WebSocket)

WebSocket broadcasts follow observer pattern:

```mermaid
graph LR
    Subject[WebSocket Hub] --> Observer1[Browser Client 1]
    Subject --> Observer2[Browser Client 2]
    Subject --> Observer3[Browser Client N]

    Event[Download Progress<br/>New Episode<br/>Status Change] --> Subject

    style Subject fill:#FFD700
    style Event fill:#90EE90
```

**Implementation**: `controllers/websockets.go`

- `HandleWebsocketMessages()`: Hub goroutine
- `Wshandler()`: Client connection handler
- `SendMessage()`: Broadcast to all clients

### 4. Singleton Pattern

Settings use singleton pattern:

```mermaid
graph TD
    Request1[Request 1] --> GetSettings[GetOrCreateSetting]
    Request2[Request 2] --> GetSettings
    Request3[Request 3] --> GetSettings

    GetSettings --> Check{Setting Exists?}
    Check -->|Yes| Return[Return Existing]
    Check -->|No| Create[Create Default]
    Create --> Return
    Return --> Cache[(Single Setting Record)]

    style GetSettings fill:#FFD700
    style Cache fill:#90EE90
```

**Implementation**: `db/dbfunctions.go:GetOrCreateSetting()`

## Data Management

### Database Schema Design

```mermaid
erDiagram
    PODCAST ||--o{ PODCAST_ITEM : contains
    PODCAST }o--o{ TAG : tagged_with
    PODCAST {
        uuid id PK
        string title
        text summary
        string author
        string image
        string url
        timestamp last_episode
        bool is_paused
        timestamp created_at
        timestamp updated_at
    }

    PODCAST_ITEM {
        uuid id PK
        uuid podcast_id FK
        string title
        text summary
        string episode_type
        int duration
        timestamp pub_date
        string file_url
        string guid
        string image
        timestamp download_date
        string download_path
        int download_status
        bool is_played
        timestamp bookmark_date
        string local_image
        int64 file_size
    }

    TAG {
        uuid id PK
        string label
        text description
    }

    PODCAST_TAGS {
        uuid podcast_id FK
        uuid tag_id FK
    }

    SETTING {
        uuid id PK
        bool download_on_add
        int initial_download_count
        bool auto_download
        bool append_date_to_filename
        bool append_episode_number_to_filename
        bool dark_mode
        bool download_episode_images
        bool generate_nfo_file
        string base_url
        int max_download_concurrency
        string user_agent
    }

    JOB_LOCK {
        uuid id PK
        timestamp date
        string name
        int duration
    }

    MIGRATION {
        uuid id PK
        timestamp date
        string name
    }

    PODCAST ||--o{ PODCAST_TAGS : has
    TAG ||--o{ PODCAST_TAGS : applied_to
```

### Transaction Management

Simple transaction strategy:

```mermaid
sequenceDiagram
    participant Service
    participant DB
    participant SQLite

    Service->>DB: Begin Transaction
    DB->>SQLite: BEGIN

    alt All Operations Succeed
        Service->>DB: Operation 1
        DB->>SQLite: INSERT/UPDATE
        Service->>DB: Operation 2
        DB->>SQLite: INSERT/UPDATE
        Service->>DB: Commit
        DB->>SQLite: COMMIT
    else Any Operation Fails
        Service->>DB: Rollback
        DB->>SQLite: ROLLBACK
    end
```

**Note**: GORM handles most transactions automatically. Explicit transactions
used for:

- Multi-podcast operations
- OPML import (multiple podcasts)
- Bulk episode updates

### File Organization

```mermaid
graph TD
    Root[/assets] --> P1[Podcast 1 Folder]
    Root --> P2[Podcast 2 Folder]
    Root --> PN[Podcast N Folder]

    P1 --> E11[Episode 1.mp3]
    P1 --> E12[Episode 2.mp3]
    P1 --> E1N[Episode N.mp3]

    P2 --> E21[Episode 1.mp3]
    P2 --> E22[Episode 2.mp3]

    style Root fill:#FFD700
    style P1 fill:#90EE90
    style P2 fill:#90EE90
```

**Naming Convention**:

```
[optional-date-prefix-][optional-episode-number-]sanitized-episode-title.mp3
```

**Example**:

- Without prefixes: `great-podcast-episode-title.mp3`
- With date: `2024-01-15-great-podcast-episode-title.mp3`
- With both: `2024-01-15-001-great-podcast-episode-title.mp3`

## Background Job Design

### Job Scheduling

```mermaid
gantt
    title Background Job Schedule (Default: CHECK_FREQUENCY=30min)
    dateFormat HH:mm
    axisFormat %H:%M

    section Frequent Jobs
    Refresh Episodes       :a1, 00:00, 30m
    Check Missing Files    :a2, 00:00, 30m
    Download Images        :a3, 00:00, 30m

    section Medium Frequency
    Unlock Missed Jobs     :b1, 00:00, 60m
    Update File Sizes      :b2, 00:00, 90m

    section Low Frequency
    Create Backup          :c1, 00:00, 2880m
```

### Job Locking Mechanism

```mermaid
sequenceDiagram
    participant Job1 as Job Instance 1
    participant Job2 as Job Instance 2
    participant DB as Job Lock Table

    Job1->>DB: Try Acquire Lock (job_name)

    alt Lock Available
        DB-->>Job1: Lock Acquired
        Job1->>Job1: Execute Job
        Job1->>DB: Release Lock
    else Lock Held
        DB-->>Job1: Lock Busy
        Job1->>Job1: Skip Execution
    end

    Note over Job1,Job2: Prevents duplicate execution

    Job2->>DB: Try Acquire Lock (same job_name)
    DB-->>Job2: Lock Busy (Job1 still running)
    Job2->>Job2: Skip Execution
```

**Implementation**: `service/podcastService.go`

- `CreateLock(name, duration)`: Acquire lock
- `ReleaseLock(name)`: Release lock
- `UnlockMissedJobs()`: Clean up stale locks

### Download Queue Management

```mermaid
graph TD
    Trigger[New Episodes Detected] --> Queue{Auto-Download?}
    Queue -->|Yes| Count[Check Queue Size]
    Queue -->|No| Manual[Wait for Manual Download]

    Count --> Limit{< Max Concurrent?}
    Limit -->|Yes| Start[Start Download]
    Limit -->|No| Wait[Wait for Slot]

    Start --> Download[Download Episode]
    Download --> Update[Update Status]
    Update --> Broadcast[Broadcast Progress]
    Broadcast --> Complete{More Episodes?}

    Complete -->|Yes| Count
    Complete -->|No| Done[All Done]

    Wait -.->|Slot Available| Start

    style Start fill:#90EE90
    style Download fill:#FFD700
    style Done fill:#90EE90
```

## Error Handling Strategy

### Error Classification

```mermaid
graph TD
    Error[Error Occurs] --> Type{Error Type}

    Type -->|Network| Network[Network Error]
    Type -->|Parse| Parse[Parse Error]
    Type -->|Database| DB[Database Error]
    Type -->|File| File[File System Error]

    Network --> Retry{Retryable?}
    Retry -->|Yes| RetryLogic[Retry with Backoff]
    Retry -->|No| Log1[Log + Continue]

    Parse --> Log2[Log + Skip Item]
    DB --> Critical[Critical Error]
    Critical --> Halt[Halt Operation]

    File --> FileCheck{File Operation}
    FileCheck -->|Download| MarkFailed[Mark Failed + Retry Later]
    FileCheck -->|Delete| Log3[Log + Continue]

    style Critical fill:#FF6B6B
    style Halt fill:#FF6B6B
    style RetryLogic fill:#FFD700
```

### Error Recovery

| Error Type               | Strategy                                       | User Impact                          |
| ------------------------ | ---------------------------------------------- | ------------------------------------ |
| RSS Feed Fetch Failed    | Log error, skip this refresh cycle             | Podcast not updated until next cycle |
| Episode Download Failed  | Mark as `NotDownloaded`, retry on next job run | Episode remains in queue             |
| Database Connection Lost | Application restart required                   | Service interruption                 |
| File Write Failed        | Log error, mark episode as failed              | Episode not available                |
| Parse Error              | Log error, skip malformed item                 | Some episodes may not be added       |

## Caching Strategy

**Current State**: No caching layer implemented

**Rationale**:

- SQLite is fast enough for read operations
- Episode files cached on disk
- RSS feeds refreshed on schedule (not real-time)
- Low concurrent user count expected

**Potential Future Caching**:

```mermaid
graph LR
    Request[Request] --> Cache{In Cache?}
    Cache -->|Hit| Return[Return Cached]
    Cache -->|Miss| DB[(Database)]
    DB --> Store[Store in Cache]
    Store --> Return

    style Cache fill:#FFD700
    style Return fill:#90EE90
```

## Concurrency Control

### Download Concurrency

```mermaid
graph TD
    Jobs[Background Jobs] --> Semaphore[Download Semaphore<br/>Size: MaxDownloadConcurrency]

    Semaphore --> W1[Worker 1<br/>Downloading]
    Semaphore --> W2[Worker 2<br/>Downloading]
    Semaphore --> W3[Worker 3<br/>Downloading]
    Semaphore --> WN[Worker N<br/>Downloading]

    W1 --> Release1[Release Slot]
    W2 --> Release2[Release Slot]
    W3 --> Release3[Release Slot]
    WN --> ReleaseN[Release Slot]

    Release1 -.-> Semaphore
    Release2 -.-> Semaphore
    Release3 -.-> Semaphore
    ReleaseN -.-> Semaphore

    Queue[Download Queue] -.->|Wait for Slot| Semaphore

    style Semaphore fill:#FFD700
    style Queue fill:#E8E8E8
```

**Implementation**: Controlled by `Setting.MaxDownloadConcurrency` (default: 5)

### Database Concurrency

**SQLite Limitations**:

- Single writer at a time
- Multiple readers allowed
- Write lock blocks all operations

**Mitigation**:

- Short transactions
- Optimistic locking via GORM
- Job locking prevents duplicate writes
- Read-heavy workload minimizes contention

## Security Design

### Authentication Flow

```mermaid
sequenceDiagram
    actor User
    participant Browser
    participant Middleware as Basic Auth Middleware
    participant Handler as Request Handler

    User->>Browser: Access Podgrab
    Browser->>Middleware: HTTP Request

    alt PASSWORD Set
        Middleware->>Browser: 401 + WWW-Authenticate
        Browser->>User: Show Login Dialog
        User->>Browser: Enter Credentials
        Browser->>Middleware: Request + Authorization Header

        alt Credentials Valid
            Middleware->>Handler: Forward Request
            Handler-->>Browser: Response
        else Credentials Invalid
            Middleware-->>Browser: 401 Unauthorized
        end
    else No PASSWORD
        Middleware->>Handler: Forward Request
        Handler-->>Browser: Response
    end
```

**Security Considerations**:

- Basic Auth over HTTP is insecure (use reverse proxy with HTTPS)
- No rate limiting (vulnerable to brute force)
- No session management (credentials sent with every request)
- Fixed username (`podgrab`) cannot be changed

### Input Sanitization

```mermaid
graph TD
    Input[User Input] --> Validate[Validation]

    Validate --> URL{Input Type}
    URL -->|URL| URLValid[URL Validation]
    URL -->|File Path| PathSanitize[Path Sanitization]
    URL -->|HTML| HTMLStrip[HTML Strip/Sanitize]
    URL -->|Text| TextValid[Text Validation]

    URLValid --> Process[Process Request]
    PathSanitize --> Process
    HTMLStrip --> Process
    TextValid --> Process

    style Validate fill:#FFD700
    style Process fill:#90EE90
```

**Sanitization Functions**:

- `internal/sanitize/sanitize.go`: Path and filename sanitization
- `bluemonday`: HTML sanitization
- `html-strip-tags-go`: HTML tag removal

## Performance Optimizations

### Database Queries

**Optimizations Applied**:

1. **Indexing**: Primary keys (UUIDs), foreign keys, timestamps
1. **Eager Loading**: `Preload()` for podcast items and tags
1. **Batch Operations**: Bulk inserts for episodes
1. **Query Limits**: Pagination support (though not heavily used)

**Query Patterns**:

```sql
-- Efficient: Uses index on podcast_id
SELECT * FROM podcast_items WHERE podcast_id = ?

-- Efficient: Uses index on created_at
SELECT * FROM podcasts ORDER BY created_at DESC

-- Potentially Slow: Full table scan for text search
SELECT * FROM podcasts WHERE title LIKE '%search%'
```

### File I/O

**Optimizations**:

- Streaming downloads (not loaded into memory)
- Concurrent downloads with semaphore
- Existing file detection (skip re-download)
- Chunked file writes

### Template Rendering

**Approach**: Server-side rendering with minimal client JavaScript

**Trade-offs**:

- ✅ Lower client CPU usage
- ✅ Works without JavaScript
- ✅ Simpler codebase
- ❌ Full page reloads for navigation
- ❌ Higher server CPU usage

## Design Trade-offs

| Decision                        | Advantages                               | Disadvantages                              |
| ------------------------------- | ---------------------------------------- | ------------------------------------------ |
| SQLite vs PostgreSQL            | Simple deployment, no separate DB server | Limited concurrency, no horizontal scaling |
| Monolith vs Microservices       | Simple deployment, easier development    | All-or-nothing scaling, tight coupling     |
| Server-side templates vs SPA    | SEO-friendly, simple                     | Less interactive, more server load         |
| Basic Auth vs OAuth             | Simple setup                             | Less secure, limited features              |
| File storage vs S3              | No cloud dependency, lower cost          | Not scalable, no CDN                       |
| Scheduled jobs vs Message Queue | Simple implementation                    | Less reliable, no guaranteed execution     |

## Related Documentation

- [Overview](overview.md) - High-level architecture
- [Data Flow](data-flow.md) - Request/response flows
- [Database Schema](database-schema.md) - Detailed schema
- [Development Setup](../development/setup.md) - Development environment

______________________________________________________________________

**Next Steps**: Review [Data Flow](data-flow.md) for detailed request/response
patterns.
