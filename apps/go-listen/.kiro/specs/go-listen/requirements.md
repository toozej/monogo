# Requirements Document

## Introduction

go-listen lets users search for artists and automatically add their top 5 songs to designated `incoming` playlists on Spotify.
The application integrates with the current scaffold for Go starter projects.
It provides a web interface and an API endpoint to manage artists.
It includes duplicate detection, playlist management, and logs of all operations.
The design follows recommended practices for web security.

## Requirements

### Requirement 1

**User Story:** I want to search for an artist by name and automatically add their top 5 songs to a selected `incoming` playlist.
This lets me quickly discover and organize new music.

#### Acceptance Criteria

1. When a user enters an artist name, the system must use fuzzy search to find the closest artist match.
2. When the system finds an artist, it must retrieve their top 5 songs from Spotify.
3. After retrieval, the system must add these songs to the `incoming` playlist that the user selected.
4. After it adds songs successfully, the system must display a confirmation with details of the added songs.
5. If no artist matches, the system must display an error message that describes the failed search.

### Requirement 2

**User Story:** I want to select from my current `incoming` playlists through a searchable dropdown.
This lets me organize music into the playlist that I select.

#### Acceptance Criteria

1. When the page loads, the system must populate the dropdown with playlists from the `Incoming` folder in Spotify.
2. When a user types in the dropdown, the system must filter playlists by the search term.
3. When a user selects a playlist, the system must display its embedded Spotify player.
4. If no `Incoming` folder exists, the system must display a message that identifies the absent folder.

### Requirement 3

**User Story:** I want to see an embedded Spotify player for my selected playlist.
This lets me immediately listen to the music that I organize.

#### Acceptance Criteria

1. When a user selects a playlist from the dropdown, the system must display its embedded Spotify player.
2. When the playlist changes, the embedded player must update to show the new playlist.
3. If the playlist is empty, the player must display a message that identifies the empty playlist.

### Requirement 4

**User Story:** I want a notification if an artist's songs already exist in a playlist, with an option to override this check.
This lets me prevent duplicates and keep control of my playlists.

#### Acceptance Criteria

1. Before it adds songs, the system must check whether the artist's top 5 songs already exist in the target playlist.
2. If songs already exist, the system must display a message with the previous addition and its timestamp.
3. When it detects duplicate songs, the system must provide an `Add Anyway` button to override the check.
4. When a user clicks this button, the system must add the songs regardless of duplicates.
5. The API must provide a force parameter to bypass duplicate detection.

### Requirement 5

**User Story:** I want to interact with the system programmatically through an API endpoint.
This lets me integrate the system with other tools and scripts.

#### Acceptance Criteria

1. For POST requests to `/api/add-artist`, the system must accept artist name and playlist parameters.
2. For valid API requests, the system must return a JSON response with operation results.
3. With the force parameter, the system must bypass duplicate detection.
4. For invalid API requests, the system must return HTTP status codes and error messages that describe the failure.

### Requirement 6

**User Story:** I want logs of all operations so that I can track usage and diagnose problems.

#### Acceptance Criteria

1. For each artist search, the system must log the search term and matched artist.
2. After it adds songs, the system must log which songs it added to which playlist.
3. When errors occur, the system must log error details at severity levels that correspond to the errors.
4. For API calls, the system must log request details and responses.

### Requirement 7

**User Story:** I want the web application to work across all my devices so that I can manage playlists from anywhere.

#### Acceptance Criteria

1. On mobile devices, the interface must be fully responsive and usable.
2. On tablets, the interface must adapt to the screen size.
3. On desktops, the interface must use the available screen space effectively.
4. When screen orientation changes, the interface must adjust accordingly.

### Requirement 8

**User Story:** I want protection against common web vulnerabilities so that my data and the system remain secure.

#### Acceptance Criteria

1. For form submissions, the system must validate all input data for injection attacks.
2. For requests that change state, the system must implement CSRF protection.
3. For received requests, the system must implement rate limits to prevent abuse.
4. For user input, the system must sanitize and validate all data.
5. If it detects suspicious activity, the system must log the activity and respond to the threat.

### Requirement 9

**User Story:** I want access without complex authentication on an internal network so that I can use the application within my trusted environment.

#### Acceptance Criteria

1. The web interface must not require user authentication.
2. API calls must not require authentication tokens.
3. At startup, the application must immediately serve requests without authentication setup.
