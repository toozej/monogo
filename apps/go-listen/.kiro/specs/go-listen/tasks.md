# Implementation Plan

- [x] 1. Set up project structure and core interfaces
  - Create directory structure for web server, services, and static files
  - Define core interfaces for Spotify service, playlist manager, and duplicate detector
  - Set up embedded static file serving with Go embed package
  - _Requirements: 1.1, 2.1, 3.1_

- [x] 2. Implement Spotify service integration
  - [x] 2.1 Create Spotify client wrapper with authentication
    - Implement SpotifyService interface with Client Credentials flow
    - Add configuration for Spotify app credentials
    - Create token refresh mechanism
    - Write unit tests for authentication flow
    - _Requirements: 1.1, 1.2, 9.3_

  - [x] 2.2 Implement artist search with fuzzy matching
    - Create FuzzyArtistSearcher with search and matching logic
    - Implement artist search API integration
    - Add fuzzy matching algorithm for artist names
    - Write unit tests for search and matching scenarios
    - _Requirements: 1.1, 1.5_

  - [x] 2.3 Implement top 5 tracks retrieval
    - Add method to get artist's top 5 tracks from Spotify API
    - Handle cases where artists have fewer than 5 tracks
    - Write unit tests for track retrieval
    - _Requirements: 1.2_

- [x] 3. Create playlist management system
  - [x] 3.1 Implement "Incoming" folder playlist retrieval
    - Create method to fetch playlists from "Incoming" folder
    - Handle cases where "Incoming" folder doesn't exist
    - Add playlist filtering and search functionality
    - Write unit tests for playlist retrieval and filtering
    - _Requirements: 2.1, 2.2, 2.4_

  - [x] 3.2 Implement playlist track addition
    - Create method to add tracks to specified playlist
    - Handle Spotify API rate limiting and errors
    - Add batch track addition for efficiency
    - Write unit tests for track addition scenarios
    - _Requirements: 1.3_

- [x] 4. Build duplicate detection system
  - [x] 4.1 Create duplicate detection logic
    - Implement method to check if artist's tracks already exist in playlist
    - Add timestamp tracking for when tracks were last added
    - Create duplicate result structure with detailed information
    - Write unit tests for duplicate detection scenarios
    - _Requirements: 4.1, 4.2_

  - [x] 4.2 Add override functionality
    - Implement force parameter to bypass duplicate detection
    - Add override button functionality for web interface
    - Handle override logic in both web and API contexts
    - Write unit tests for override scenarios
    - _Requirements: 4.3, 4.4, 5.3_

- [x] 5. Develop web server and HTTP handlers
  - [x] 5.1 Create HTTP server with Cobra integration
    - Add "serve" command to existing Cobra CLI structure
    - Implement server startup and shutdown logic
    - Add server configuration through Viper
    - Write integration tests for server lifecycle
    - _Requirements: 9.3_

  - [x] 5.2 Implement security middleware
    - Add CSRF protection for state-changing operations
    - Implement rate limiting per IP address
    - Create input validation and sanitization middleware
    - Add security headers and protection against common attacks
    - Write unit tests for security middleware
    - _Requirements: 8.1, 8.2, 8.3, 8.4, 8.5_

  - [x] 5.3 Create web interface handlers
    - Implement main page handler serving the web interface
    - Add handler for playlist retrieval and filtering
    - Create handler for artist addition with duplicate checking
    - Add handler for embedded player URL generation
    - Write unit tests for all web handlers
    - _Requirements: 1.1, 2.1, 3.1, 4.1_

- [x] 6. Build REST API endpoints
  - [x] 6.1 Implement /api/add-artist endpoint
    - Create POST handler for programmatic artist addition
    - Add JSON request/response handling with validation
    - Implement force parameter for duplicate bypass
    - Add proper HTTP status codes and error responses
    - Write unit tests for API endpoint scenarios
    - _Requirements: 5.1, 5.2, 5.4_

  - [x] 6.2 Add playlist management API endpoints
    - Create GET endpoint for retrieving "Incoming" playlists
    - Add playlist search/filtering API endpoint
    - Implement proper JSON responses with playlist data
    - Write unit tests for playlist API endpoints
    - _Requirements: 2.1, 2.2_

- [x] 7. Create responsive web interface
  - [x] 7.1 Build HTML structure and responsive layout
    - Create main HTML page with semantic structure
    - Implement mobile-first responsive CSS with breakpoints
    - Add viewport meta tags and responsive design elements
    - Test layout on different screen sizes and orientations
    - _Requirements: 7.1, 7.2, 7.3, 7.4_

  - [x] 7.2 Implement interactive JavaScript functionality
    - Add artist search form with validation
    - Create searchable dropdown for playlist selection
    - Implement AJAX calls for artist addition and playlist retrieval
    - Add loading states and user feedback messages
    - Write JavaScript unit tests for interactive functionality
    - _Requirements: 1.1, 1.4, 2.2, 4.2_

  - [x] 7.3 Integrate Spotify embedded player
    - Add embedded player iframe with dynamic playlist updates
    - Implement player URL generation based on selected playlist
    - Handle empty playlist states with appropriate messaging
    - Test player functionality across different devices
    - _Requirements: 3.1, 3.2, 3.3_

- [x] 8. Implement comprehensive logging system
  - [x] 8.1 Set up structured logging with multiple levels
    - Configure logrus with structured JSON logging
    - Add logging middleware for HTTP requests
    - Implement log correlation IDs for request tracking
    - Write unit tests for logging functionality
    - _Requirements: 6.1, 6.2, 6.3, 6.4_

  - [x] 8.2 Add operation-specific logging
    - Log all artist searches with terms and matched results
    - Log all track additions with playlist and song details
    - Log duplicate detections and override usage
    - Log security events and suspicious activity
    - Write integration tests for logging scenarios
    - _Requirements: 6.1, 6.2, 6.3, 6.4_

- [x] 9. Create comprehensive test suite
  - [x] 9.1 Write unit tests for all components
    - Test Spotify service integration with mocked API responses
    - Test fuzzy matching algorithms with various input scenarios
    - Test duplicate detection logic with different playlist states
    - Test security middleware with various attack scenarios
    - _Requirements: All requirements through comprehensive testing_

  - [x] 9.2 Create integration tests
    - Test complete request flows from web interface to Spotify API
    - Test API endpoints with real request/response cycles
    - Test error handling and recovery scenarios
    - Test rate limiting and security protection mechanisms
    - _Requirements: All requirements through end-to-end testing_

- [x] 10. Final integration and configuration
  - [x] 10.1 Wire all components together
    - Integrate all services into the main server structure
    - Add proper dependency injection and configuration
    - Ensure all handlers use the correct services and middleware
    - Test complete application functionality
    - _Requirements: All requirements through final integration_

  - [x] 10.2 Add configuration and deployment setup
    - Extend Viper configuration with all new server settings
    - Add environment variable configuration for Spotify credentials
    - Create example configuration files and documentation
    - Test application startup and configuration loading
    - _Requirements: 9.3, plus operational requirements_

- [x] 11. Ensure application works with project scaffolding
  - [x] 11.1 Ensure pre-commit checks run successfully
    - Run pre-commit checks with `make pre-commit`
    - Ensure pre-commit checks run successfully, fixing their issues if not
    - Briefly document usage of Makefile in README.md as the primary method of developing and installing application locally
    - _Requirements: All requirements through final integration_

    - [x] 11.2 Ensure Docker images can be built successfully
    - Ensure Docker images can be built successfully, fixing their issues if not
    - _Requirements: All requirements through final integration_

    - [x] 11.3 Document API and test functionality
    - Write REST API documentation in [API Documentation](docs/api.md)
    - Test complete application functionality
    - _Requirements: All requirements through final integration_
- [-] 12. Utilize github.com/zmb3/spotify/v2 library for core Spotify functionality
  - [x] 12.1 Ensure user is authenticated to Spotify
    - Ensure user is authenticated to Spotify, resolving error "Failed to get user playlists error=\"Valid user authentication required\""
    - Continue to use "github.com/zmb3/spotify/v2/auth" package, and follow example from https://github.com/zmb3/spotify/blob/master/examples/authenticate/authcode/authenticate.go as necessary
    - Ensure application can fetch the user playlists to validate that user authentication to Spotify is functional
    - _Requirements: 1.1, 1.2, 9.3_
  - [x] 12.2 Use github.com/zmb3/spotify/v2 library where appropriate for interactions with Spotify
    - Use github.com/zmb3/spotify/v2 library where appropriate for interactions with Spotify, particularly in `internal/services/spotify/*.go` files service.go and client.go
    - follow example from https://github.com/zmb3/spotify/blob/master/examples/features/features.go as necessary
    - Ensure application can fetch the user playlists to validate that user authentication to Spotify is functional
    - _Requirements: 1.1, 1.2, 2.1, 2.3, 9.3_
  - [x] 12.3 Resolve web page issues
    - Web page should not be auto-refreshing parts such as playlist drop-down, etc. after a playlist has been selected
    - Web page should not be auto-refreshing the entire page after the initial load of playlists and the Spotify web player have loaded
    - _Requirements: 1.1, 1.2, 2.1, 2.3, 9.3_

- [x] 13. Fix test failures and improve code quality
  - [x] 13.1 Fix Spotify service test signature issues
    - Update test files in `internal/services/spotify/` to match the new service constructor signature
    - Fix `NewService` calls in test files to use `config.SpotifyConfig` instead of individual string parameters
    - Ensure all Spotify service tests pass
    - _Requirements: All requirements through comprehensive testing_
  - [x] 13.2 Fix configuration test expectations
    - Update config tests to match actual default host behavior (127.0.0.1 vs localhost)
    - Ensure configuration validation tests pass
    - _Requirements: 9.3_
  - [x] 13.3 Validate complete application functionality
    - Run full test suite and ensure all tests pass
    - Test authentication flow end-to-end
    - Verify web interface works correctly with user authentication
    - Update documentation as necessary for changes made during tasks 12 and 13
    - _Requirements: All requirements through final integration_
