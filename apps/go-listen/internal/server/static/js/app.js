// go-listen web interface JavaScript
class GoListenApp {
    constructor() {
        this.form = document.getElementById('artist-form');
        this.artistInput = document.getElementById('artist-name');
        this.playlistSelect = document.getElementById('playlist-select');
        this.addButton = document.getElementById('add-button');
        this.overrideButton = document.getElementById('override-button');
        this.messageArea = document.getElementById('message-area');
        this.playerArea = document.getElementById('spotify-player');

        // Scraping form elements
        this.scrapeForm = document.getElementById('scrape-form');
        this.scrapeUrlInput = document.getElementById('scrape-url');
        this.cssSelectorInput = document.getElementById('css-selector');
        this.scrapePlaylistSelect = document.getElementById('scrape-playlist-select');
        this.scrapeButton = document.getElementById('scrape-button');
        this.scrapeMessageArea = document.getElementById('scrape-message-area');
        this.scrapeResults = document.getElementById('scrape-results');

        // State management
        this.playlists = [];
        this.isLoading = false;
        this.isScraping = false;
        this.isUpdatingDropdown = false; // Flag to prevent unwanted player updates
        this.csrfToken = null;

        this.init();
    }

    init() {
        this.checkAuthStatus();
        this.setupEventListeners();
        this.setupFormValidation();
        this.setupDeviceOptimizations();
        this.fetchCSRFToken();
    }

    setupDeviceOptimizations() {
        const deviceInfo = this.testPlayerCompatibility();

        // Add device-specific classes for CSS targeting
        document.body.classList.add(
            deviceInfo.isMobile ? 'device-mobile' :
                deviceInfo.isTablet ? 'device-tablet' : 'device-desktop'
        );

        // Optimize player for mobile devices
        if (deviceInfo.isMobile) {
            // Reduce player height on mobile for better UX
            const style = document.createElement('style');
            style.textContent = `
                .player-container iframe {
                    height: 300px;
                }
                .player-container {
                    min-height: 300px;
                }
            `;
            document.head.appendChild(style);
        }

        // Log device compatibility for debugging
        console.log('go-listen initialized for device:', deviceInfo);
    }

    setupEventListeners() {
        // Form submission - prevent any default form submission behavior
        this.form.addEventListener('submit', (e) => {
            e.preventDefault();
            e.stopPropagation();
            this.handleSubmit(e);
        });

        // Additional safety net to prevent form submission
        this.form.addEventListener('keydown', (e) => {
            if (e.key === 'Enter' && e.target.tagName !== 'BUTTON') {
                e.preventDefault();
                e.stopPropagation();
                this.handleSubmit(e);
            }
        });

        // Prevent any accidental form submission
        this.addButton.addEventListener('click', (e) => {
            e.preventDefault();
            e.stopPropagation();
            this.handleSubmit(e);
        });

        // Playlist selection
        this.playlistSelect.addEventListener('change', (e) => {
            // Only update player if we're not currently updating the dropdown
            if (!this.isUpdatingDropdown) {
                this.updatePlayer();
            }
        });

        // Override button
        this.overrideButton.addEventListener('click', () => this.handleOverride());

        // Artist input validation
        this.artistInput.addEventListener('input', () => this.validateArtistInput());
        this.artistInput.addEventListener('blur', () => this.validateArtistInput());

        // Keyboard navigation
        this.setupKeyboardNavigation();

        // Handle form reset
        this.form.addEventListener('reset', () => this.resetForm());

        // Scraping form event listeners
        this.scrapeForm.addEventListener('submit', (e) => {
            e.preventDefault();
            e.stopPropagation();
            this.handleScrapeSubmit(e);
        });

        this.scrapeForm.addEventListener('keydown', (e) => {
            if (e.key === 'Enter' && e.target.tagName !== 'BUTTON') {
                e.preventDefault();
                e.stopPropagation();
                this.handleScrapeSubmit(e);
            }
        });

        this.scrapeButton.addEventListener('click', (e) => {
            e.preventDefault();
            e.stopPropagation();
            this.handleScrapeSubmit(e);
        });

        // Scrape URL validation
        this.scrapeUrlInput.addEventListener('input', () => this.validateScrapeUrl());
        this.scrapeUrlInput.addEventListener('blur', () => this.validateScrapeUrl());
    }

    setupFormValidation() {
        // Set up real-time validation
        this.artistInput.setAttribute('minlength', '1');
        this.artistInput.setAttribute('maxlength', '100');

        // Custom validation messages
        this.artistInput.addEventListener('invalid', (e) => {
            if (e.target.validity.valueMissing) {
                e.target.setCustomValidity('Please enter an artist name');
            } else if (e.target.validity.tooShort) {
                e.target.setCustomValidity('Artist name must be at least 1 character');
            } else if (e.target.validity.tooLong) {
                e.target.setCustomValidity('Artist name must be less than 100 characters');
            } else {
                e.target.setCustomValidity('');
            }
        });

        this.playlistSelect.addEventListener('invalid', (e) => {
            if (e.target.validity.valueMissing) {
                e.target.setCustomValidity('Please select a playlist');
            } else {
                e.target.setCustomValidity('');
            }
        });
    }

    setupKeyboardNavigation() {
        // Handle Enter key on override button
        this.overrideButton.addEventListener('keydown', (e) => {
            if (e.key === 'Enter' || e.key === ' ') {
                e.preventDefault();
                this.handleOverride();
            }
        });

        // Handle Escape key to close messages
        document.addEventListener('keydown', (e) => {
            if (e.key === 'Escape' && this.messageArea.style.display !== 'none') {
                this.hideMessage();
            }
        });
    }

    async fetchCSRFToken() {
        try {
            const response = await fetch('/api/csrf-token', {
                method: 'GET',
                headers: {
                    'Accept': 'application/json',
                },
            });

            if (response.ok) {
                const data = await response.json();
                this.csrfToken = data.csrf_token;
                console.log('CSRF token fetched successfully');
            } else {
                console.warn('Failed to fetch CSRF token:', response.status);
            }
        } catch (error) {
            console.error('Error fetching CSRF token:', error);
        }
    }

    async checkAuthStatus() {
        try {
            const response = await fetch('/api/auth-status', {
                method: 'GET',
                headers: {
                    'Accept': 'application/json',
                },
            });

            if (!response.ok) {
                throw new Error(`HTTP ${response.status}: ${response.statusText}`);
            }

            const data = await response.json();

            if (data.success) {
                if (data.data.authenticated) {
                    // User is authenticated, load playlists
                    this.loadPlaylists();
                } else {
                    // User needs to authenticate
                    this.showAuthenticationRequired(data.data.auth_url);
                }
            } else {
                throw new Error(data.error || 'Failed to check authentication status');
            }
        } catch (error) {
            console.error('Failed to check auth status:', error);
            this.showMessage(`Error checking authentication: ${error.message}`, 'error');
        }
    }

    showAuthenticationRequired(authUrl) {
        // Hide the main form
        this.form.style.display = 'none';

        // Show authentication message
        const authMessage = document.createElement('div');
        authMessage.className = 'auth-required';

        const authContent = document.createElement('div');
        authContent.className = 'auth-content';

        // Create Spotify icon
        const spotifyIcon = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
        spotifyIcon.setAttribute('class', 'spotify-icon large');
        spotifyIcon.setAttribute('viewBox', '0 0 24 24');
        spotifyIcon.setAttribute('aria-hidden', 'true');
        const iconPath = document.createElementNS('http://www.w3.org/2000/svg', 'path');
        iconPath.setAttribute('fill', 'currentColor');
        iconPath.setAttribute('d', 'M12 0C5.4 0 0 5.4 0 12s5.4 12 12 12 12-5.4 12-12S18.66 0 12 0zm5.521 17.34c-.24.359-.66.48-1.021.24-2.82-1.74-6.36-2.101-10.561-1.141-.418.122-.779-.179-.899-.539-.12-.421.18-.78.54-.9 4.56-1.021 8.52-.6 11.64 1.32.42.18.479.659.301 1.02zm1.44-3.3c-.301.42-.841.6-1.262.3-3.239-1.98-8.159-2.58-11.939-1.38-.479.12-1.02-.12-1.14-.6-.12-.48.12-1.021.6-1.141C9.6 9.9 15 10.561 18.72 12.84c.361.181.54.78.241 1.2zm.12-3.36C15.24 8.4 8.82 8.16 5.16 9.301c-.6.179-1.2-.181-1.38-.721-.18-.601.18-1.2.72-1.381 4.26-1.26 11.28-1.02 15.721 1.621.539.3.719 1.02.42 1.56-.299.421-1.02.599-1.559.3z');
        spotifyIcon.appendChild(iconPath);

        // Create title
        const title = document.createElement('h2');
        title.textContent = 'Spotify Authentication Required';

        // Create description
        const description = document.createElement('p');
        description.textContent = 'To access your playlists and add artists, you need to authenticate with Spotify.';

        // Create auth button
        const authButton = document.createElement('a');
        authButton.href = authUrl;
        authButton.className = 'btn btn-primary auth-button';

        // Create small Spotify icon for button
        const smallIcon = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
        smallIcon.setAttribute('class', 'spotify-icon small');
        smallIcon.setAttribute('viewBox', '0 0 24 24');
        smallIcon.setAttribute('aria-hidden', 'true');
        const smallIconPath = document.createElementNS('http://www.w3.org/2000/svg', 'path');
        smallIconPath.setAttribute('fill', 'currentColor');
        smallIconPath.setAttribute('d', 'M12 0C5.4 0 0 5.4 0 12s5.4 12 12 12 12-5.4 12-12S18.66 0 12 0zm5.521 17.34c-.24.359-.66.48-1.021.24-2.82-1.74-6.36-2.101-10.561-1.141-.418.122-.779-.179-.899-.539-.12-.421.18-.78.54-.9 4.56-1.021 8.52-.6 11.64 1.32.42.18.479.659.301 1.02zm1.44-3.3c-.301.42-.841.6-1.262.3-3.239-1.98-8.159-2.58-11.939-1.38-.479.12-1.02-.12-1.14-.6-.12-.48.12-1.021.6-1.141C9.6 9.9 15 10.561 18.72 12.84c.361.181.54.78.241 1.2zm.12-3.36C15.24 8.4 8.82 8.16 5.16 9.301c-.6.179-1.2-.181-1.38-.721-.18-.601.18-1.2.72-1.381 4.26-1.26 11.28-1.02 15.721 1.621.539.3.719 1.02.42 1.56-.299.421-1.02.599-1.559.3z');
        smallIcon.appendChild(smallIconPath);

        authButton.appendChild(smallIcon);
        authButton.appendChild(document.createTextNode(' Connect to Spotify'));

        // Create help text
        const helpText = document.createElement('p');
        helpText.className = 'auth-help';
        helpText.textContent = 'This will open Spotify\'s authorization page in a new window.';

        // Assemble the content
        authContent.appendChild(spotifyIcon);
        authContent.appendChild(title);
        authContent.appendChild(description);
        authContent.appendChild(authButton);
        authContent.appendChild(helpText);
        authMessage.appendChild(authContent);

        // Insert before the form
        this.form.parentNode.insertBefore(authMessage, this.form);

        // Hide the player section as well
        this.playerArea.innerHTML = '';

        const placeholderDiv = document.createElement('div');
        placeholderDiv.className = 'player-placeholder';

        // Create Spotify icon for player placeholder
        const playerSpotifyIcon = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
        playerSpotifyIcon.setAttribute('class', 'spotify-icon');
        playerSpotifyIcon.setAttribute('viewBox', '0 0 24 24');
        playerSpotifyIcon.setAttribute('aria-hidden', 'true');
        const playerIconPath = document.createElementNS('http://www.w3.org/2000/svg', 'path');
        playerIconPath.setAttribute('fill', 'currentColor');
        playerIconPath.setAttribute('d', 'M12 0C5.4 0 0 5.4 0 12s5.4 12 12 12 12-5.4 12-12S18.66 0 12 0zm5.521 17.34c-.24.359-.66.48-1.021.24-2.82-1.74-6.36-2.101-10.561-1.141-.418.122-.779-.179-.899-.539-.12-.421.18-.78.54-.9 4.56-1.021 8.52-.6 11.64 1.32.42.18.479.659.301 1.02zm1.44-3.3c-.301.42-.841.6-1.262.3-3.239-1.98-8.159-2.58-11.939-1.38-.479.12-1.02-.12-1.14-.6-.12-.48.12-1.021.6-1.141C9.6 9.9 15 10.561 18.72 12.84c.361.181.54.78.241 1.2zm.12-3.36C15.24 8.4 8.82 8.16 5.16 9.301c-.6.179-1.2-.181-1.38-.721-.18-.601.18-1.2.72-1.381 4.26-1.26 11.28-1.02 15.721 1.621.539.3.719 1.02.42 1.56-.299.421-1.02.599-1.559.3z');
        playerSpotifyIcon.appendChild(playerIconPath);

        // Create message
        const message = document.createElement('p');
        message.textContent = 'Authentication required to access Spotify playlists';

        placeholderDiv.appendChild(playerSpotifyIcon);
        placeholderDiv.appendChild(message);
        this.playerArea.appendChild(placeholderDiv);
    }

    async loadPlaylists() {
        this.setPlaylistLoading(true);

        try {
            const response = await fetch('/api/playlists', {
                method: 'GET',
                headers: {
                    'Accept': 'application/json',
                },
            });

            if (!response.ok) {
                // Check if it's an authentication error
                if (response.status === 401 || response.status === 403) {
                    // Re-check auth status
                    this.checkAuthStatus();
                    return;
                }
                throw new Error(`HTTP ${response.status}: ${response.statusText}`);
            }

            const data = await response.json();

            if (data.success && Array.isArray(data.data)) {
                this.playlists = data.data;
                this.populatePlaylistSelect(this.playlists);

                if (this.playlists.length === 0) {
                    this.showMessage('No playlists found in your "Incoming" folder. Please create some playlists first.', 'warning');
                }
            } else {
                throw new Error(data.error || 'Invalid response format');
            }
        } catch (error) {
            console.error('Failed to load playlists:', error);

            // Check if it's an authentication error
            if (error.message.includes('authentication') || error.message.includes('401') || error.message.includes('403')) {
                this.checkAuthStatus();
            } else {
                this.showMessage(`Error loading playlists: ${error.message}`, 'error');
                this.populatePlaylistSelect([]);
            }
        } finally {
            this.setPlaylistLoading(false);
        }
    }

    populatePlaylistSelect(playlists) {
        // Set flag to prevent change event from triggering updatePlayer
        this.isUpdatingDropdown = true;

        // Preserve current selection to prevent unwanted resets
        const currentSelection = this.playlistSelect.value;
        const currentScrapeSelection = this.scrapePlaylistSelect.value;

        // Clear existing options for both dropdowns
        this.playlistSelect.innerHTML = '';
        this.scrapePlaylistSelect.innerHTML = '';

        if (playlists.length === 0) {
            const option = document.createElement('option');
            option.value = '';
            option.textContent = 'No playlists available';
            option.disabled = true;
            this.playlistSelect.appendChild(option);
            this.playlistSelect.disabled = true;

            const scrapeOption = option.cloneNode(true);
            this.scrapePlaylistSelect.appendChild(scrapeOption);
            this.scrapePlaylistSelect.disabled = true;
            this.isUpdatingDropdown = false;
            return;
        }

        // Add default option
        const defaultOption = document.createElement('option');
        defaultOption.value = '';
        defaultOption.textContent = 'Select a playlist...';
        this.playlistSelect.appendChild(defaultOption);

        const scrapeDefaultOption = defaultOption.cloneNode(true);
        this.scrapePlaylistSelect.appendChild(scrapeDefaultOption);

        // Add playlist options to both dropdowns
        playlists.forEach(playlist => {
            const option = document.createElement('option');
            option.value = playlist.id;
            option.textContent = `${playlist.name} (${playlist.track_count} tracks)`;
            option.dataset.embedUrl = playlist.embed_url;
            option.dataset.name = playlist.name.toLowerCase();
            this.playlistSelect.appendChild(option);

            const scrapeOption = option.cloneNode(true);
            scrapeOption.dataset.embedUrl = playlist.embed_url;
            scrapeOption.dataset.name = playlist.name.toLowerCase();
            this.scrapePlaylistSelect.appendChild(scrapeOption);
        });

        // Always enable the dropdowns if we have playlists
        this.playlistSelect.disabled = false;
        this.scrapePlaylistSelect.disabled = false;

        // Restore previous selections if they still exist
        if (currentSelection) {
            const optionExists = Array.from(this.playlistSelect.options).some(option => option.value === currentSelection);
            if (optionExists) {
                this.playlistSelect.value = currentSelection;
            }
        }

        if (currentScrapeSelection) {
            const optionExists = Array.from(this.scrapePlaylistSelect.options).some(option => option.value === currentScrapeSelection);
            if (optionExists) {
                this.scrapePlaylistSelect.value = currentScrapeSelection;
            }
        }

        // Clear the flag after a short delay to allow DOM to settle
        setTimeout(() => {
            this.isUpdatingDropdown = false;
        }, 50);
    }

    // Playlist search functionality removed - using simple dropdown selection

    setPlaylistLoading(loading) {
        if (loading) {
            this.playlistSelect.innerHTML = '';
            const option = document.createElement('option');
            option.value = '';
            option.textContent = 'Loading playlists...';
            this.playlistSelect.appendChild(option);
            this.playlistSelect.disabled = true;
        }
        // Note: enabling is handled in populatePlaylistSelect
    }

    async handleSubmit(e) {
        if (e) {
            e.preventDefault();
            e.stopPropagation();
        }

        // Prevent multiple simultaneous submissions
        if (this.isLoading) {
            return;
        }

        // Clear any existing messages
        this.hideMessage();

        // Validate form
        if (!this.validateForm()) {
            return;
        }

        const artistName = this.artistInput.value.trim();
        const playlistId = this.playlistSelect.value;

        await this.addArtist(artistName, playlistId, false);
    }

    async handleOverride() {
        if (!this.validateForm()) {
            return;
        }

        const artistName = this.artistInput.value.trim();
        const playlistId = this.playlistSelect.value;

        await this.addArtist(artistName, playlistId, true);
    }

    validateForm() {
        let isValid = true;

        // Validate artist name
        const artistName = this.artistInput.value.trim();
        if (!artistName) {
            this.showFieldError(this.artistInput, 'Artist name is required');
            isValid = false;
        } else if (artistName.length > 100) {
            this.showFieldError(this.artistInput, 'Artist name must be less than 100 characters');
            isValid = false;
        } else {
            this.clearFieldError(this.artistInput);
        }

        // Validate playlist selection
        const playlistId = this.playlistSelect.value;
        if (!playlistId) {
            this.showFieldError(this.playlistSelect, 'Please select a playlist');
            isValid = false;
        } else {
            this.clearFieldError(this.playlistSelect);
        }

        return isValid;
    }

    validateArtistInput() {
        const artistName = this.artistInput.value.trim();

        if (artistName && artistName.length > 100) {
            this.showFieldError(this.artistInput, 'Artist name must be less than 100 characters');
        } else {
            this.clearFieldError(this.artistInput);
        }

        // Update button state
        this.updateButtonState();
    }

    updateButtonState() {
        const artistName = this.artistInput.value.trim();
        const playlistId = this.playlistSelect.value;
        const isFormValid = artistName && playlistId && artistName.length <= 100;

        this.addButton.disabled = !isFormValid || this.isLoading;

        // Ensure playlist dropdown stays enabled if we have playlists
        if (this.playlists.length > 0 && !this.isLoading) {
            this.playlistSelect.disabled = false;
        }
    }

    showFieldError(field, message) {
        field.setCustomValidity(message);
        field.classList.add('error');

        // Show browser validation message
        if (!field.validity.valid) {
            field.reportValidity();
        }
    }

    clearFieldError(field) {
        field.setCustomValidity('');
        field.classList.remove('error');
    }

    resetForm() {
        this.clearFieldError(this.artistInput);
        this.clearFieldError(this.playlistSelect);
        this.hideMessage();
        this.overrideButton.style.display = 'none';
        this.updateButtonState();
    }

    async addArtist(artistName, playlistId, force = false) {
        this.setLoading(true);
        this.hideMessage();

        try {
            // Get selected playlist name for better user feedback
            const selectedOption = this.playlistSelect.selectedOptions[0];
            const playlistName = selectedOption ? selectedOption.textContent.split(' (')[0] : 'selected playlist';

            const headers = {
                'Content-Type': 'application/json',
                'Accept': 'application/json',
            };

            // Add CSRF token if available
            if (this.csrfToken) {
                headers['X-CSRF-Token'] = this.csrfToken;
            }

            const response = await fetch('/api/add-artist', {
                method: 'POST',
                headers: headers,
                body: JSON.stringify({
                    artist_name: artistName,
                    playlist_id: playlistId,
                    force: force
                })
            });

            // Handle HTTP errors
            if (!response.ok) {
                let errorMessage = `HTTP ${response.status}: ${response.statusText}`;

                try {
                    const errorData = await response.json();
                    if (errorData.error) {
                        errorMessage = errorData.error;
                    }
                } catch (e) {
                    // Use default HTTP error message if JSON parsing fails
                }

                throw new Error(errorMessage);
            }

            const data = await response.json();

            if (data.success) {
                const message = data.message || `Successfully added ${artistName} to ${playlistName}`;
                this.showMessage(message, 'success', 5000);
                this.overrideButton.style.display = 'none';

                // Clear form on success - but keep playlist selection
                this.artistInput.value = '';
                this.updateButtonState();

                // Ensure playlist dropdown remains enabled and selected
                if (this.playlists.length > 0) {
                    this.playlistSelect.disabled = false;
                }

                // Refresh player to show new tracks
                this.refreshPlayer();

                // Log success for debugging
                console.log('Artist added successfully:', data.data);

            } else if (data.is_duplicate && !force) {
                const message = data.message || `${artistName} may already be in ${playlistName}`;
                this.showMessage(message, 'warning');
                this.overrideButton.style.display = 'inline-block';
                this.overrideButton.focus(); // Focus override button for accessibility

            } else {
                const message = data.error || data.message || 'Failed to add artist';
                this.showMessage(message, 'error');
                this.overrideButton.style.display = 'none';
            }

        } catch (error) {
            console.error('Error adding artist:', error);

            let userMessage = 'Error adding artist';
            if (error.message.includes('Failed to fetch')) {
                userMessage = 'Network error. Please check your connection and try again.';
            } else if (error.message.includes('HTTP 429')) {
                userMessage = 'Too many requests. Please wait a moment and try again.';
            } else if (error.message.includes('HTTP 500')) {
                userMessage = 'Server error. Please try again later.';
            } else {
                userMessage = `Error: ${error.message}`;
            }

            this.showMessage(userMessage, 'error');
            this.overrideButton.style.display = 'none';

        } finally {
            this.setLoading(false);
        }
    }

    updatePlayer() {
        const selectedOption = this.playlistSelect.selectedOptions[0];

        if (selectedOption && selectedOption.dataset.embedUrl) {
            const embedUrl = selectedOption.dataset.embedUrl;
            const playlistName = selectedOption.textContent.split(' (')[0];
            const trackCount = this.extractTrackCount(selectedOption.textContent);

            // Show loading state while iframe loads
            this.showPlayerLoading(playlistName);

            // Create iframe with enhanced attributes for better compatibility
            const iframe = document.createElement('iframe');
            iframe.src = embedUrl;
            iframe.width = '100%';
            iframe.height = '380';
            iframe.frameBorder = '0';
            iframe.allowTransparency = 'true';
            iframe.allow = 'encrypted-media';
            iframe.title = `Spotify playlist player for ${playlistName}`;
            iframe.loading = 'lazy';

            // Handle iframe load events
            iframe.addEventListener('load', () => {
                this.hidePlayerLoading();
                console.log(`Spotify player loaded for playlist: ${playlistName}`);
            });

            iframe.addEventListener('error', () => {
                this.showPlayerError(playlistName);
                console.error(`Failed to load Spotify player for playlist: ${playlistName}`);
            });

            // Replace content with iframe
            this.playerArea.innerHTML = '';
            this.playerArea.appendChild(iframe);

            // Handle empty playlist state
            if (trackCount === 0) {
                this.showEmptyPlaylistMessage(playlistName);
            }

        } else {
            this.showPlayerPlaceholder();
        }
    }

    showPlayerLoading(playlistName) {
        this.playerArea.innerHTML = '';

        const loadingDiv = document.createElement('div');
        loadingDiv.className = 'player-loading';

        const spinner = document.createElement('div');
        spinner.className = 'loading-spinner';

        const message = document.createElement('p');
        message.textContent = `Loading ${playlistName}...`;

        loadingDiv.appendChild(spinner);
        loadingDiv.appendChild(message);
        this.playerArea.appendChild(loadingDiv);
    }

    hidePlayerLoading() {
        const loadingElement = this.playerArea.querySelector('.player-loading');
        if (loadingElement) {
            loadingElement.style.display = 'none';
        }
    }

    showPlayerError(playlistName) {
        this.playerArea.innerHTML = '';

        const errorDiv = document.createElement('div');
        errorDiv.className = 'player-error';

        // Create error icon
        const errorIcon = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
        errorIcon.setAttribute('class', 'error-icon');
        errorIcon.setAttribute('viewBox', '0 0 24 24');
        errorIcon.setAttribute('aria-hidden', 'true');
        const iconPath = document.createElementNS('http://www.w3.org/2000/svg', 'path');
        iconPath.setAttribute('fill', 'currentColor');
        iconPath.setAttribute('d', 'M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm-2 15l-5-5 1.41-1.41L10 14.17l7.59-7.59L19 8l-9 9z');
        errorIcon.appendChild(iconPath);

        // Create error message
        const errorMessage = document.createElement('p');
        errorMessage.textContent = `Failed to load player for ${playlistName}`;

        // Create retry button
        const retryButton = document.createElement('button');
        retryButton.type = 'button';
        retryButton.className = 'btn btn-primary';
        retryButton.textContent = 'Retry';
        retryButton.onclick = () => window.goListenApp.updatePlayer();

        errorDiv.appendChild(errorIcon);
        errorDiv.appendChild(errorMessage);
        errorDiv.appendChild(retryButton);
        this.playerArea.appendChild(errorDiv);
    }

    showEmptyPlaylistMessage(playlistName) {
        // Add overlay message for empty playlists
        const overlay = document.createElement('div');
        overlay.className = 'player-overlay';

        const messageDiv = document.createElement('div');
        messageDiv.className = 'empty-playlist-message';

        // Create music icon
        const musicIcon = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
        musicIcon.setAttribute('class', 'music-icon');
        musicIcon.setAttribute('viewBox', '0 0 24 24');
        musicIcon.setAttribute('aria-hidden', 'true');
        const iconPath = document.createElementNS('http://www.w3.org/2000/svg', 'path');
        iconPath.setAttribute('fill', 'currentColor');
        iconPath.setAttribute('d', 'M12 3v10.55c-.59-.34-1.27-.55-2-.55-2.21 0-4 1.79-4 4s1.79 4 4 4 4-1.79 4-4V7h4V3h-6z');
        musicIcon.appendChild(iconPath);

        // Create messages
        const titleMessage = document.createElement('p');
        const strongElement = document.createElement('strong');
        strongElement.textContent = playlistName;
        titleMessage.appendChild(strongElement);
        titleMessage.appendChild(document.createTextNode(' is empty'));

        const helpMessage = document.createElement('p');
        helpMessage.textContent = 'Add some artists to start building your playlist!';

        messageDiv.appendChild(musicIcon);
        messageDiv.appendChild(titleMessage);
        messageDiv.appendChild(helpMessage);
        overlay.appendChild(messageDiv);

        // Position overlay over the iframe
        this.playerArea.style.position = 'relative';
        this.playerArea.appendChild(overlay);
    }

    showPlayerPlaceholder() {
        this.playerArea.innerHTML = '';

        const placeholderDiv = document.createElement('div');
        placeholderDiv.className = 'player-placeholder';

        // Create Spotify icon
        const spotifyIcon = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
        spotifyIcon.setAttribute('class', 'spotify-icon');
        spotifyIcon.setAttribute('viewBox', '0 0 24 24');
        spotifyIcon.setAttribute('aria-hidden', 'true');
        const iconPath = document.createElementNS('http://www.w3.org/2000/svg', 'path');
        iconPath.setAttribute('fill', 'currentColor');
        iconPath.setAttribute('d', 'M12 0C5.4 0 0 5.4 0 12s5.4 12 12 12 12-5.4 12-12S18.66 0 12 0zm5.521 17.34c-.24.359-.66.48-1.021.24-2.82-1.74-6.36-2.101-10.561-1.141-.418.122-.779-.179-.899-.539-.12-.421.18-.78.54-.9 4.56-1.021 8.52-.6 11.64 1.32.42.18.479.659.301 1.02zm1.44-3.3c-.301.42-.841.6-1.262.3-3.239-1.98-8.159-2.58-11.939-1.38-.479.12-1.02-.12-1.14-.6-.12-.48.12-1.021.6-1.141C9.6 9.9 15 10.561 18.72 12.84c.361.181.54.78.241 1.2zm.12-3.36C15.24 8.4 8.82 8.16 5.16 9.301c-.6.179-1.2-.181-1.38-.721-.18-.601.18-1.2.72-1.381 4.26-1.26 11.28-1.02 15.721 1.621.539.3.719 1.02.42 1.56-.299.421-1.02.599-1.559.3z');
        spotifyIcon.appendChild(iconPath);

        // Create message
        const message = document.createElement('p');
        message.textContent = 'Select a playlist to see the embedded player';

        // Create help text
        const helpText = document.createElement('small');
        helpText.textContent = 'Choose from your "Incoming" folder playlists above';

        placeholderDiv.appendChild(spotifyIcon);
        placeholderDiv.appendChild(message);
        placeholderDiv.appendChild(helpText);
        this.playerArea.appendChild(placeholderDiv);
    }

    extractTrackCount(optionText) {
        const match = optionText.match(/\((\d+) tracks?\)/);
        return match ? parseInt(match[1], 10) : 0;
    }

    generateEmbedURL(playlistURI) {
        // Enhanced URL generation with additional parameters for better embedding
        if (playlistURI && playlistURI.startsWith('spotify:playlist:')) {
            const playlistID = playlistURI.replace('spotify:playlist:', '');
            return `https://open.spotify.com/embed/playlist/${playlistID}?utm_source=generator&theme=0`;
        }
        return null;
    }

    refreshPlayer() {
        // Only refresh the current player without reloading all playlists
        // This prevents unwanted dropdown refreshing
        const currentSelection = this.playlistSelect.value;
        if (currentSelection) {
            // Force refresh the embedded player iframe to show new tracks
            setTimeout(() => {
                this.updatePlayer();
            }, 1000);
        }
    }

    // Method to test player functionality across different devices
    testPlayerCompatibility() {
        const userAgent = navigator.userAgent;
        const isMobile = /Android|webOS|iPhone|iPad|iPod|BlackBerry|IEMobile|Opera Mini/i.test(userAgent);
        const isTablet = /iPad|Android(?=.*Mobile)/i.test(userAgent);
        const isDesktop = !isMobile && !isTablet;

        console.log('Device compatibility:', {
            isMobile,
            isTablet,
            isDesktop,
            userAgent,
            supportsIframe: 'HTMLIFrameElement' in window,
            supportsEmbeddedMedia: 'allow' in document.createElement('iframe')
        });

        return {
            isMobile,
            isTablet,
            isDesktop,
            compatible: 'HTMLIFrameElement' in window
        };
    }

    showMessage(message, type, autoHideMs = 0) {
        this.messageArea.textContent = message;
        this.messageArea.className = `message-area ${type}`;
        this.messageArea.style.display = 'block';

        // Scroll message into view if needed
        this.messageArea.scrollIntoView({ behavior: 'smooth', block: 'nearest' });

        // Auto-hide messages after specified time
        if (autoHideMs > 0) {
            setTimeout(() => {
                this.hideMessage();
            }, autoHideMs);
        }

        // Announce to screen readers
        this.messageArea.setAttribute('aria-live', type === 'error' ? 'assertive' : 'polite');
    }

    hideMessage() {
        this.messageArea.style.display = 'none';
        this.messageArea.textContent = '';
        this.messageArea.className = 'message-area';
    }

    async handleScrapeSubmit(e) {
        if (e) {
            e.preventDefault();
            e.stopPropagation();
        }

        // Prevent multiple simultaneous submissions
        if (this.isScraping) {
            return;
        }

        // Clear any existing messages and results
        this.hideScrapeMessage();
        this.hideScrapeResults();

        // Validate form
        if (!this.validateScrapeForm()) {
            return;
        }

        const url = this.scrapeUrlInput.value.trim();
        const cssSelector = this.cssSelectorInput.value.trim();
        const playlistId = this.scrapePlaylistSelect.value;

        await this.scrapeAndAddArtists(url, cssSelector, playlistId);
    }

    validateScrapeForm() {
        let isValid = true;

        // Validate URL
        const url = this.scrapeUrlInput.value.trim();
        if (!url) {
            this.showFieldError(this.scrapeUrlInput, 'URL is required');
            isValid = false;
        } else if (!this.isValidUrl(url)) {
            this.showFieldError(this.scrapeUrlInput, 'Please enter a valid URL');
            isValid = false;
        } else {
            this.clearFieldError(this.scrapeUrlInput);
        }

        // Validate playlist selection
        const playlistId = this.scrapePlaylistSelect.value;
        if (!playlistId) {
            this.showFieldError(this.scrapePlaylistSelect, 'Please select a playlist');
            isValid = false;
        } else {
            this.clearFieldError(this.scrapePlaylistSelect);
        }

        return isValid;
    }

    validateScrapeUrl() {
        const url = this.scrapeUrlInput.value.trim();

        if (url && !this.isValidUrl(url)) {
            this.showFieldError(this.scrapeUrlInput, 'Please enter a valid URL');
        } else {
            this.clearFieldError(this.scrapeUrlInput);
        }
    }

    isValidUrl(string) {
        try {
            const url = new URL(string);
            return url.protocol === 'http:' || url.protocol === 'https:';
        } catch (_) {
            return false;
        }
    }

    async scrapeAndAddArtists(url, cssSelector, playlistId) {
        this.setScrapeLoading(true);
        this.hideScrapeMessage();
        this.hideScrapeResults();

        try {
            const selectedOption = this.scrapePlaylistSelect.selectedOptions[0];
            const playlistName = selectedOption ? selectedOption.textContent.split(' (')[0] : 'selected playlist';

            const headers = {
                'Content-Type': 'application/json',
                'Accept': 'application/json',
            };

            // Add CSRF token if available
            if (this.csrfToken) {
                headers['X-CSRF-Token'] = this.csrfToken;
            }

            const response = await fetch('/api/scrape-artists', {
                method: 'POST',
                headers: headers,
                body: JSON.stringify({
                    url: url,
                    css_selector: cssSelector,
                    playlist_id: playlistId,
                    force: false
                })
            });

            // Handle HTTP errors
            if (!response.ok) {
                let errorMessage = `HTTP ${response.status}: ${response.statusText}`;

                try {
                    const errorData = await response.json();
                    if (errorData.error) {
                        errorMessage = errorData.error;
                    }
                } catch (e) {
                    // Use default HTTP error message if JSON parsing fails
                }

                throw new Error(errorMessage);
            }

            const data = await response.json();

            if (data.success && data.data) {
                const result = data.data;
                this.displayScrapeResults(result, playlistName);
                
                // Show success message
                const message = `Found ${result.artists_found.length} artists, successfully added ${result.success_count} to ${playlistName}`;
                this.showScrapeMessage(message, 'success', 5000);

                // Clear form on success
                this.scrapeUrlInput.value = '';
                this.cssSelectorInput.value = '';

                // Refresh player to show new tracks
                this.refreshPlayer();

            } else {
                const message = data.error || 'Failed to scrape artists';
                this.showScrapeMessage(message, 'error');
            }

        } catch (error) {
            console.error('Error scraping artists:', error);

            let userMessage = 'Error scraping artists';
            if (error.message.includes('Failed to fetch')) {
                userMessage = 'Network error. Please check your connection and try again.';
            } else if (error.message.includes('HTTP 429')) {
                userMessage = 'Too many requests. Please wait a moment and try again.';
            } else if (error.message.includes('HTTP 500')) {
                userMessage = 'Server error. Please try again later.';
            } else {
                userMessage = `Error: ${error.message}`;
            }

            this.showScrapeMessage(userMessage, 'error');

        } finally {
            this.setScrapeLoading(false);
        }
    }

    displayScrapeResults(result, playlistName) {
        // Clear previous results
        this.scrapeResults.innerHTML = '';

        // Create summary section
        const summary = document.createElement('div');
        summary.className = 'scrape-summary';

        const summaryTitle = document.createElement('h3');
        summaryTitle.textContent = 'Scraping Results';
        summary.appendChild(summaryTitle);

        const stats = document.createElement('div');
        stats.className = 'scrape-summary-stats';

        // Success count
        const successStat = this.createStatElement('Success', result.success_count, 'success');
        stats.appendChild(successStat);

        // Failure count
        if (result.failure_count > 0) {
            const failureStat = this.createStatElement('Failed', result.failure_count, 'error');
            stats.appendChild(failureStat);
        }

        // Duplicate count
        if (result.duplicate_count > 0) {
            const duplicateStat = this.createStatElement('Duplicates', result.duplicate_count, 'duplicate');
            stats.appendChild(duplicateStat);
        }

        // Total tracks added
        const tracksStat = this.createStatElement('Tracks Added', result.total_tracks_added, 'success');
        stats.appendChild(tracksStat);

        summary.appendChild(stats);
        this.scrapeResults.appendChild(summary);

        // Create artist results section
        if (result.match_results && result.match_results.length > 0) {
            const artistResults = document.createElement('div');
            artistResults.className = 'artist-results';

            result.match_results.forEach(match => {
                const artistResult = this.createArtistResultElement(match);
                artistResults.appendChild(artistResult);
            });

            this.scrapeResults.appendChild(artistResults);
        }

        // Show results
        this.scrapeResults.classList.add('visible');
        this.scrapeResults.scrollIntoView({ behavior: 'smooth', block: 'nearest' });
    }

    createStatElement(label, value, type) {
        const stat = document.createElement('div');
        stat.className = 'scrape-stat';

        const statLabel = document.createElement('div');
        statLabel.className = 'scrape-stat-label';
        statLabel.textContent = label;

        const statValue = document.createElement('div');
        statValue.className = `scrape-stat-value ${type}`;
        statValue.textContent = value;

        stat.appendChild(statLabel);
        stat.appendChild(statValue);

        return stat;
    }

    createArtistResultElement(match) {
        const result = document.createElement('div');
        
        let status = 'error';
        let iconPath = 'M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm1 15h-2v-2h2v2zm0-4h-2V7h2v6z'; // Error icon
        
        if (match.was_duplicate) {
            status = 'duplicate';
            iconPath = 'M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm-2 15l-5-5 1.41-1.41L10 14.17l7.59-7.59L19 8l-9 9z'; // Check icon
        } else if (match.matched && match.tracks_added > 0) {
            status = 'success';
            iconPath = 'M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm-2 15l-5-5 1.41-1.41L10 14.17l7.59-7.59L19 8l-9 9z'; // Check icon
        }
        
        result.className = `artist-result ${status}`;

        // Icon
        const icon = document.createElement('div');
        icon.className = 'artist-result-icon';
        const svg = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
        svg.setAttribute('viewBox', '0 0 24 24');
        svg.setAttribute('aria-hidden', 'true');
        const path = document.createElementNS('http://www.w3.org/2000/svg', 'path');
        path.setAttribute('fill', 'currentColor');
        path.setAttribute('d', iconPath);
        svg.appendChild(path);
        icon.appendChild(svg);
        result.appendChild(icon);

        // Content
        const content = document.createElement('div');
        content.className = 'artist-result-content';

        const name = document.createElement('div');
        name.className = 'artist-result-name';
        name.textContent = match.artist ? match.artist.name : match.query;

        const statusText = document.createElement('div');
        statusText.className = 'artist-result-status';
        
        if (match.was_duplicate) {
            statusText.textContent = 'Already in playlist (skipped)';
        } else if (match.matched && match.tracks_added > 0) {
            statusText.textContent = `Successfully added ${match.tracks_added} tracks`;
        } else if (match.error) {
            statusText.textContent = match.error;
        } else {
            statusText.textContent = 'Not found on Spotify';
        }

        content.appendChild(name);
        content.appendChild(statusText);

        if (match.confidence > 0) {
            const confidence = document.createElement('div');
            confidence.className = 'artist-result-tracks';
            confidence.textContent = `Match confidence: ${(match.confidence * 100).toFixed(0)}%`;
            content.appendChild(confidence);
        }

        result.appendChild(content);

        return result;
    }

    showScrapeMessage(message, type, autoHideMs = 0) {
        this.scrapeMessageArea.textContent = message;
        this.scrapeMessageArea.className = `message-area ${type}`;
        this.scrapeMessageArea.style.display = 'block';

        // Scroll message into view if needed
        this.scrapeMessageArea.scrollIntoView({ behavior: 'smooth', block: 'nearest' });

        // Auto-hide messages after specified time
        if (autoHideMs > 0) {
            setTimeout(() => {
                this.hideScrapeMessage();
            }, autoHideMs);
        }

        // Announce to screen readers
        this.scrapeMessageArea.setAttribute('aria-live', type === 'error' ? 'assertive' : 'polite');
    }

    hideScrapeMessage() {
        this.scrapeMessageArea.style.display = 'none';
        this.scrapeMessageArea.textContent = '';
        this.scrapeMessageArea.className = 'message-area';
    }

    hideScrapeResults() {
        this.scrapeResults.classList.remove('visible');
        this.scrapeResults.innerHTML = '';
    }

    setScrapeLoading(loading) {
        this.isScraping = loading;

        if (loading) {
            this.scrapeButton.classList.add('loading');
            this.scrapeForm.classList.add('loading');
            this.scrapeUrlInput.disabled = true;
            this.cssSelectorInput.disabled = true;
            this.scrapePlaylistSelect.disabled = true;
        } else {
            this.scrapeButton.classList.remove('loading');
            this.scrapeForm.classList.remove('loading');
            this.scrapeUrlInput.disabled = false;
            this.cssSelectorInput.disabled = false;
            this.scrapePlaylistSelect.disabled = this.playlists.length === 0;
        }
    }

    setLoading(loading) {
        this.isLoading = loading;

        if (loading) {
            this.addButton.classList.add('loading');
            this.form.classList.add('loading');
            this.artistInput.disabled = true;
            // Don't disable playlist select during loading to prevent graying out
            this.overrideButton.disabled = true;
        } else {
            this.addButton.classList.remove('loading');
            this.form.classList.remove('loading');
            this.artistInput.disabled = false;
            // Only disable playlist select if no playlists are available
            this.playlistSelect.disabled = this.playlists.length === 0;
            this.overrideButton.disabled = false;
        }

        this.updateButtonState();
    }
}

// Utility functions for better user experience
class UIUtils {
    static debounce(func, wait) {
        let timeout;
        return function executedFunction(...args) {
            const later = () => {
                clearTimeout(timeout);
                func(...args);
            };
            clearTimeout(timeout);
            timeout = setTimeout(later, wait);
        };
    }

    static throttle(func, limit) {
        let inThrottle;
        return function () {
            const args = arguments;
            const context = this;
            if (!inThrottle) {
                func.apply(context, args);
                inThrottle = true;
                setTimeout(() => inThrottle = false, limit);
            }
        };
    }

    static sanitizeHTML(str) {
        const temp = document.createElement('div');
        temp.textContent = str;
        return temp.innerHTML;
    }
}

// Error handling and logging
class ErrorHandler {
    static log(error, context = '') {
        const prefix = context ? `[GoListen - ${context}]:` : '[GoListen]:';
        console.error(prefix);
        console.error(error);

        // In production, you might want to send errors to a logging service
        if (window.location.hostname !== 'localhost' && window.location.hostname !== '127.0.0.1') {
            // Example: Send to logging service
            // this.sendToLoggingService(error, context);
        }
    }

    static handleNetworkError(error) {
        if (error.name === 'TypeError' && error.message.includes('Failed to fetch')) {
            return 'Network error. Please check your connection and try again.';
        }
        return error.message;
    }
}

// Store app instance globally for debugging and page visibility handling
let goListenApp = null;

// Initialize the app when the DOM is loaded
document.addEventListener('DOMContentLoaded', () => {
    try {
        goListenApp = new GoListenApp();
        window.goListenApp = goListenApp; // Make available globally
    } catch (error) {
        ErrorHandler.log(error, 'Initialization');

        // Show fallback error message
        const messageArea = document.getElementById('message-area');
        if (messageArea) {
            messageArea.textContent = 'Failed to initialize the application. Please refresh the page.';
            messageArea.className = 'message-area error';
            messageArea.style.display = 'block';
        }
    }
});

// Handle page visibility changes - removed auto-refresh to prevent unwanted reloading
// Users can manually refresh if needed
document.addEventListener('visibilitychange', () => {
    // No automatic playlist reloading to prevent dropdown refreshing
    // This was causing unwanted auto-refresh behavior
});

// Global error handler
window.addEventListener('error', (event) => {
    ErrorHandler.log(event.error, 'Global');
});

window.addEventListener('unhandledrejection', (event) => {
    ErrorHandler.log(event.reason, 'Unhandled Promise');
});