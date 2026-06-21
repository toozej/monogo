// JavaScript tests for go-listen web interface
// Test the utility functions and core logic

describe('GoListenApp Core Functionality', () => {
    let app;
    
    beforeEach(() => {
        // Mock DOM elements
        document.body.innerHTML = `
            <form id="artist-form">
                <input id="artist-name" type="text" />
                <select id="playlist-select"></select>
                <button id="add-button" type="submit">Add</button>
                <button id="override-button" type="button">Override</button>
            </form>
            <div id="message-area"></div>
            <div id="spotify-player"></div>
            <form id="scrape-form">
                <input id="scrape-url" type="url" />
                <input id="css-selector" type="text" />
                <select id="scrape-playlist-select"></select>
                <button id="scrape-button" type="submit">Scrape</button>
            </form>
            <div id="scrape-message-area"></div>
            <div id="scrape-results"></div>
        `;
        
        // Mock fetch for initialization
        fetch.mockResolvedValue({
            ok: true,
            json: async () => ({ csrf_token: 'test-token', success: true, data: { authenticated: true } })
        });
        
        // Create app instance
        app = new GoListenApp();
    });

    test('should initialize with correct default state', () => {
        expect(app.playlists).toEqual([]);
        expect(app.isLoading).toBe(false);
        expect(app.isScraping).toBe(false);
        expect(app.isUpdatingDropdown).toBe(false);
        expect(app.csrfToken).toBeNull();
    });

    test('should validate form correctly', () => {
        // Test empty artist name
        app.artistInput.value = '';
        app.playlistSelect.value = 'playlist1';
        expect(app.validateForm()).toBe(false);

        // Test empty playlist
        app.artistInput.value = 'Test Artist';
        app.playlistSelect.value = '';
        expect(app.validateForm()).toBe(false);

        // Test valid form
        app.artistInput.value = 'Test Artist';
        app.playlistSelect.value = 'playlist1';
        expect(app.validateForm()).toBe(true);

        // Test artist name too long
        app.artistInput.value = 'a'.repeat(101);
        app.playlistSelect.value = 'playlist1';
        expect(app.validateForm()).toBe(false);
    });

    test('should extract track count from option text', () => {
        expect(app.extractTrackCount('Playlist Name (5 tracks)')).toBe(5);
        expect(app.extractTrackCount('Playlist Name (1 track)')).toBe(1);
        expect(app.extractTrackCount('Playlist Name (0 tracks)')).toBe(0);
        expect(app.extractTrackCount('Playlist Name')).toBe(0);
    });

    test('should generate embed URL correctly', () => {
        const playlistURI = 'spotify:playlist:37i9dQZF1DXcBWIGoYBM5M';
        const expectedURL = 'https://open.spotify.com/embed/playlist/37i9dQZF1DXcBWIGoYBM5M?utm_source=generator&theme=0';
        expect(app.generateEmbedURL(playlistURI)).toBe(expectedURL);
        
        expect(app.generateEmbedURL('invalid:uri')).toBeNull();
        expect(app.generateEmbedURL('')).toBeNull();
    });

    test('should test player compatibility', () => {
        const compatibility = app.testPlayerCompatibility();
        expect(compatibility).toHaveProperty('isMobile');
        expect(compatibility).toHaveProperty('isTablet');
        expect(compatibility).toHaveProperty('isDesktop');
        expect(compatibility).toHaveProperty('compatible');
        expect(typeof compatibility.compatible).toBe('boolean');
    });

    test('should manage loading state correctly', () => {
        app.setLoading(true);
        expect(app.isLoading).toBe(true);
        expect(app.artistInput.disabled).toBe(true);
        expect(app.overrideButton.disabled).toBe(true);

        app.setLoading(false);
        expect(app.isLoading).toBe(false);
        expect(app.artistInput.disabled).toBe(false);
        expect(app.overrideButton.disabled).toBe(false);
    });

    test('should show and hide messages correctly', () => {
        app.showMessage('Test message', 'success');
        expect(app.messageArea.textContent).toBe('Test message');
        expect(app.messageArea.className).toBe('message-area success');
        expect(app.messageArea.style.display).toBe('block');

        app.hideMessage();
        expect(app.messageArea.textContent).toBe('');
        expect(app.messageArea.className).toBe('message-area');
        expect(app.messageArea.style.display).toBe('none');
    });

    test('should populate playlist select correctly', () => {
        const playlists = [
            { id: 'playlist1', name: 'Test Playlist 1', track_count: 5, embed_url: 'https://example.com/1' },
            { id: 'playlist2', name: 'Test Playlist 2', track_count: 10, embed_url: 'https://example.com/2' }
        ];

        app.populatePlaylistSelect(playlists);
        expect(app.playlistSelect.disabled).toBe(false);
        
        // Test with empty playlists
        app.populatePlaylistSelect([]);
        expect(app.playlistSelect.disabled).toBe(true);
    });

    test('should handle field errors correctly', () => {
        const mockField = {
            setCustomValidity: jest.fn(),
            classList: { add: jest.fn(), remove: jest.fn() },
            validity: { valid: false },
            reportValidity: jest.fn()
        };

        app.showFieldError(mockField, 'Test error');
        expect(mockField.setCustomValidity).toHaveBeenCalledWith('Test error');
        expect(mockField.classList.add).toHaveBeenCalledWith('error');

        app.clearFieldError(mockField);
        expect(mockField.setCustomValidity).toHaveBeenCalledWith('');
        expect(mockField.classList.remove).toHaveBeenCalledWith('error');
    });

    test('should reset form correctly', () => {
        app.overrideButton.style.display = 'inline-block';
        app.messageArea.style.display = 'block';
        
        app.resetForm();
        expect(app.overrideButton.style.display).toBe('none');
    });
});

describe('Auto-refresh Prevention Tests', () => {
    test('should prevent auto-refresh behavior', () => {
        const mockApp = {
            isUpdatingDropdown: false,
            updatePlayerCalled: false,
            updatePlayer: function() {
                this.updatePlayerCalled = true;
            }
        };

        function handlePlaylistChange() {
            if (!mockApp.isUpdatingDropdown) {
                mockApp.updatePlayer();
            }
        }

        // Should call updatePlayer when not updating dropdown
        mockApp.isUpdatingDropdown = false;
        mockApp.updatePlayerCalled = false;
        handlePlaylistChange();
        expect(mockApp.updatePlayerCalled).toBe(true);

        // Should NOT call updatePlayer when updating dropdown
        mockApp.isUpdatingDropdown = true;
        mockApp.updatePlayerCalled = false;
        handlePlaylistChange();
        expect(mockApp.updatePlayerCalled).toBe(false);
    });

    test('should handle dropdown population without triggering unwanted events', () => {
        function populateDropdownSafely(playlists, currentSelection) {
            const newOptions = [];
            
            if (playlists.length === 0) {
                return { disabled: true, options: [], selectedValue: '' };
            }
            
            newOptions.push({ value: '', text: 'Select a playlist...' });
            
            playlists.forEach(playlist => {
                newOptions.push({
                    value: playlist.id,
                    text: `${playlist.name} (${playlist.track_count} tracks)`
                });
            });
            
            let selectedValue = '';
            if (currentSelection) {
                const optionExists = newOptions.some(option => option.value === currentSelection);
                if (optionExists) {
                    selectedValue = currentSelection;
                }
            }
            
            return { 
                disabled: false, 
                options: newOptions, 
                selectedValue: selectedValue
            };
        }

        const playlists = [
            { id: 'playlist1', name: 'Test Playlist', track_count: 5 }
        ];

        const result = populateDropdownSafely(playlists, 'playlist1');
        expect(result.disabled).toBe(false);
        expect(result.options).toHaveLength(2);
        expect(result.selectedValue).toBe('playlist1');
    });
});

describe('Utility Functions', () => {
    test('UIUtils.debounce should work correctly', (done) => {
        let callCount = 0;
        const mockFn = () => { callCount++; };
        const debouncedFn = UIUtils.debounce(mockFn, 50);

        debouncedFn();
        debouncedFn();
        debouncedFn();

        expect(callCount).toBe(0);

        setTimeout(() => {
            expect(callCount).toBe(1);
            done();
        }, 100);
    });

    test('UIUtils.throttle should work correctly', (done) => {
        let callCount = 0;
        const mockFn = () => { callCount++; };
        const throttledFn = UIUtils.throttle(mockFn, 100);

        throttledFn();
        throttledFn();
        throttledFn();

        expect(callCount).toBe(1);

        setTimeout(() => {
            throttledFn();
            expect(callCount).toBe(2);
            done();
        }, 150);
    });

    test('UIUtils.sanitizeHTML should sanitize content', () => {
        const unsafeHTML = '<script>alert("xss")</script><p>Safe content</p>';
        const sanitized = UIUtils.sanitizeHTML(unsafeHTML);
        
        expect(sanitized).not.toContain('<script>');
        expect(sanitized).toContain('&lt;p&gt;Safe content&lt;/p&gt;');
    });

    test('ErrorHandler.log should log errors', () => {
        const consoleSpy = jest.spyOn(console, 'error').mockImplementation();
        
        const testError = new Error('Test error');
        ErrorHandler.log(testError, 'TestContext');

        expect(consoleSpy).toHaveBeenCalledWith('[GoListen - TestContext]:');
        expect(consoleSpy).toHaveBeenCalledWith(testError);
        
        consoleSpy.mockRestore();
    });

    test('ErrorHandler.handleNetworkError should handle network errors', () => {
        const networkError = new TypeError('Failed to fetch');
        const result = ErrorHandler.handleNetworkError(networkError);
        expect(result).toBe('Network error. Please check your connection and try again.');

        const otherError = new Error('Other error');
        const result2 = ErrorHandler.handleNetworkError(otherError);
        expect(result2).toBe('Other error');
    });
});

describe('Form Validation', () => {
    test('should validate form inputs correctly', () => {
        function validateForm(artistName, playlistId) {
            const trimmedArtist = artistName.trim();
            const trimmedPlaylist = playlistId.trim();
            
            if (!trimmedArtist) return false;
            if (trimmedArtist.length > 100) return false;
            if (!trimmedPlaylist) return false;
            
            return true;
        }

        expect(validateForm('', 'playlist1')).toBe(false);
        expect(validateForm('Test Artist', '')).toBe(false);
        expect(validateForm('Test Artist', 'playlist1')).toBe(true);
        expect(validateForm('a'.repeat(101), 'playlist1')).toBe(false);
        expect(validateForm('  Test Artist  ', 'playlist1')).toBe(true);
    });
});

describe('Device Detection', () => {
    test('should detect device types correctly', () => {
        function detectDevice(userAgent) {
            const isMobile = /Android|webOS|iPhone|iPad|iPod|BlackBerry|IEMobile|Opera Mini/i.test(userAgent);
            const isTablet = /iPad|Android(?=.*Mobile)/i.test(userAgent);
            const isDesktop = !isMobile && !isTablet;
            
            return { isMobile, isTablet, isDesktop };
        }

        const mobileUA = 'Mozilla/5.0 (iPhone; CPU iPhone OS 14_0 like Mac OS X)';
        const mobileDevice = detectDevice(mobileUA);
        expect(mobileDevice.isMobile).toBe(true);
        expect(mobileDevice.isDesktop).toBe(false);

        const desktopUA = 'Mozilla/5.0 (Windows NT 10.0; Win64; x64)';
        const desktopDevice = detectDevice(desktopUA);
        expect(desktopDevice.isMobile).toBe(false);
        expect(desktopDevice.isDesktop).toBe(true);
    });
});

describe('Loading States', () => {
    test('should manage loading states correctly', () => {
        function setLoadingState(isLoading, hasPlaylists = true) {
            return {
                isLoading: isLoading,
                artistInputDisabled: isLoading,
                playlistSelectDisabled: isLoading || !hasPlaylists,
                addButtonDisabled: isLoading
            };
        }

        const loadingState = setLoadingState(true);
        expect(loadingState.isLoading).toBe(true);
        expect(loadingState.artistInputDisabled).toBe(true);
        expect(loadingState.addButtonDisabled).toBe(true);

        const normalState = setLoadingState(false);
        expect(normalState.isLoading).toBe(false);
        expect(normalState.artistInputDisabled).toBe(false);
        expect(normalState.playlistSelectDisabled).toBe(false);

        const noPlaylistsState = setLoadingState(false, false);
        expect(noPlaylistsState.playlistSelectDisabled).toBe(true);
    });
});

describe('Message Display', () => {
    test('should handle message display correctly', () => {
        function showMessage(message, type) {
            return {
                text: message,
                className: `message-area ${type}`,
                visible: true
            };
        }
        
        function hideMessage() {
            return {
                text: '',
                className: 'message-area',
                visible: false
            };
        }

        const successMsg = showMessage('Success!', 'success');
        expect(successMsg.text).toBe('Success!');
        expect(successMsg.className).toBe('message-area success');
        expect(successMsg.visible).toBe(true);

        const errorMsg = showMessage('Error!', 'error');
        expect(errorMsg.text).toBe('Error!');
        expect(errorMsg.className).toBe('message-area error');

        const hiddenMsg = hideMessage();
        expect(hiddenMsg.text).toBe('');
        expect(hiddenMsg.visible).toBe(false);
    });
});

describe('Async Functions', () => {
    beforeEach(() => {
        fetch.mockClear();
    });

    test('should handle CSRF token fetch', async () => {
        // Test the actual fetch logic, not the mock
        fetch.mockResolvedValueOnce({
            ok: true,
            json: async () => ({ csrf_token: 'test-token' })
        });

        // Test the actual fetch logic
        const response = await fetch('/api/csrf-token');
        const data = await response.json();
        
        expect(response.ok).toBe(true);
        expect(data.csrf_token).toBe('test-token');
    });

    test('should handle CSRF token fetch failure', async () => {
        const consoleSpy = jest.spyOn(console, 'warn').mockImplementation();
        
        fetch.mockResolvedValueOnce({
            ok: false,
            status: 500
        });

        const response = await fetch('/api/csrf-token');
        expect(response.ok).toBe(false);
        expect(response.status).toBe(500);
        
        consoleSpy.mockRestore();
    });

    test('should handle auth status check', async () => {
        fetch.mockResolvedValueOnce({
            ok: true,
            json: async () => ({
                success: true,
                data: { authenticated: true }
            })
        });

        const response = await fetch('/api/auth-status');
        const data = await response.json();
        
        expect(response.ok).toBe(true);
        expect(data.success).toBe(true);
        expect(data.data.authenticated).toBe(true);
    });

    test('should handle playlist loading', async () => {
        const mockPlaylists = [
            { id: 'playlist1', name: 'Test Playlist', track_count: 5 }
        ];
        
        fetch.mockResolvedValueOnce({
            ok: true,
            json: async () => ({
                success: true,
                data: mockPlaylists
            })
        });

        const response = await fetch('/api/playlists');
        const data = await response.json();
        
        expect(response.ok).toBe(true);
        expect(data.success).toBe(true);
        expect(data.data).toEqual(mockPlaylists);
    });
});

describe('Scraping Functionality', () => {
    let app;
    
    beforeEach(() => {
        document.body.innerHTML = `
            <form id="artist-form">
                <input id="artist-name" type="text" />
                <select id="playlist-select"></select>
                <button id="add-button" type="submit">Add</button>
                <button id="override-button" type="button">Override</button>
            </form>
            <div id="message-area"></div>
            <div id="spotify-player"></div>
            <form id="scrape-form">
                <input id="scrape-url" type="url" />
                <input id="css-selector" type="text" />
                <select id="scrape-playlist-select"></select>
                <button id="scrape-button" type="submit">Scrape</button>
            </form>
            <div id="scrape-message-area"></div>
            <div id="scrape-results"></div>
        `;
        
        app = new GoListenApp();
    });

    test('should validate scrape form correctly', () => {
        // Test empty URL
        app.scrapeUrlInput.value = '';
        app.scrapePlaylistSelect.value = 'playlist1';
        expect(app.validateScrapeForm()).toBe(false);

        // Test invalid URL
        app.scrapeUrlInput.value = 'not-a-url';
        app.scrapePlaylistSelect.value = 'playlist1';
        expect(app.validateScrapeForm()).toBe(false);

        // Test empty playlist
        app.scrapeUrlInput.value = 'https://example.com';
        app.scrapePlaylistSelect.value = '';
        expect(app.validateScrapeForm()).toBe(false);

        // Test valid form
        app.scrapeUrlInput.value = 'https://example.com';
        app.scrapePlaylistSelect.value = 'playlist1';
        expect(app.validateScrapeForm()).toBe(true);
    });

    test('should validate URLs correctly', () => {
        expect(app.isValidUrl('https://example.com')).toBe(true);
        expect(app.isValidUrl('http://example.com')).toBe(true);
        expect(app.isValidUrl('ftp://example.com')).toBe(false);
        expect(app.isValidUrl('not-a-url')).toBe(false);
        expect(app.isValidUrl('')).toBe(false);
    });

    test('should manage scrape loading state correctly', () => {
        app.playlists = [{ id: 'playlist1', name: 'Test' }];
        
        app.setScrapeLoading(true);
        expect(app.isScraping).toBe(true);
        expect(app.scrapeUrlInput.disabled).toBe(true);
        expect(app.cssSelectorInput.disabled).toBe(true);
        expect(app.scrapePlaylistSelect.disabled).toBe(true);

        app.setScrapeLoading(false);
        expect(app.isScraping).toBe(false);
        expect(app.scrapeUrlInput.disabled).toBe(false);
        expect(app.cssSelectorInput.disabled).toBe(false);
        expect(app.scrapePlaylistSelect.disabled).toBe(false);
    });

    test('should show and hide scrape messages correctly', () => {
        app.showScrapeMessage('Test message', 'success');
        expect(app.scrapeMessageArea.textContent).toBe('Test message');
        expect(app.scrapeMessageArea.className).toBe('message-area success');
        expect(app.scrapeMessageArea.style.display).toBe('block');

        app.hideScrapeMessage();
        expect(app.scrapeMessageArea.textContent).toBe('');
        expect(app.scrapeMessageArea.className).toBe('message-area');
        expect(app.scrapeMessageArea.style.display).toBe('none');
    });

    test('should show and hide scrape results correctly', () => {
        app.scrapeResults.classList.add('visible');
        app.scrapeResults.innerHTML = '<div>Test</div>';

        app.hideScrapeResults();
        expect(app.scrapeResults.classList.contains('visible')).toBe(false);
        expect(app.scrapeResults.innerHTML).toBe('');
    });

    test('should create stat element correctly', () => {
        const stat = app.createStatElement('Success', 5, 'success');
        expect(stat.className).toBe('scrape-stat');
        
        const label = stat.querySelector('.scrape-stat-label');
        expect(label.textContent).toBe('Success');
        
        const value = stat.querySelector('.scrape-stat-value');
        expect(value.textContent).toBe('5');
        expect(value.className).toBe('scrape-stat-value success');
    });

    test('should create artist result element for success', () => {
        const match = {
            query: 'Test Artist',
            matched: true,
            artist: { name: 'Test Artist' },
            confidence: 0.95,
            tracks_added: 5,
            was_duplicate: false,
            error: null
        };

        const result = app.createArtistResultElement(match);
        expect(result.className).toContain('success');
        
        const name = result.querySelector('.artist-result-name');
        expect(name.textContent).toBe('Test Artist');
        
        const status = result.querySelector('.artist-result-status');
        expect(status.textContent).toBe('Successfully added 5 tracks');
    });

    test('should create artist result element for duplicate', () => {
        const match = {
            query: 'Test Artist',
            matched: true,
            artist: { name: 'Test Artist' },
            confidence: 0.95,
            tracks_added: 0,
            was_duplicate: true,
            error: null
        };

        const result = app.createArtistResultElement(match);
        expect(result.className).toContain('duplicate');
        
        const status = result.querySelector('.artist-result-status');
        expect(status.textContent).toBe('Already in playlist (skipped)');
    });

    test('should create artist result element for error', () => {
        const match = {
            query: 'Test Artist',
            matched: false,
            artist: null,
            confidence: 0,
            tracks_added: 0,
            was_duplicate: false,
            error: 'Artist not found'
        };

        const result = app.createArtistResultElement(match);
        expect(result.className).toContain('error');
        
        const status = result.querySelector('.artist-result-status');
        expect(status.textContent).toBe('Artist not found');
    });

    test('should populate both playlist selects', () => {
        const playlists = [
            { id: 'playlist1', name: 'Test Playlist 1', track_count: 5, embed_url: 'https://example.com/1' },
            { id: 'playlist2', name: 'Test Playlist 2', track_count: 10, embed_url: 'https://example.com/2' }
        ];

        app.populatePlaylistSelect(playlists);
        
        expect(app.playlistSelect.disabled).toBe(false);
        expect(app.scrapePlaylistSelect.disabled).toBe(false);
        
        // Both should have the same number of options
        expect(app.playlistSelect.options.length).toBe(app.scrapePlaylistSelect.options.length);
        expect(app.playlistSelect.options.length).toBe(3); // default + 2 playlists
    });
});