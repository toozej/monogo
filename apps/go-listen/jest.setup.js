// Jest setup file for go-listen JavaScript tests

// Mock global objects that are not available in Jest environment
global.fetch = jest.fn();

// Mock GoListenApp class for tests
global.GoListenApp = class MockGoListenApp {
    constructor() {
        this.playlists = [];
        this.filteredPlaylists = [];
        this.isLoading = false;
        this.isScraping = false;
        this.searchTimeout = null;
        this.isUpdatingDropdown = false;
        this.csrfToken = null;
        
        // Mock DOM elements
        this.form = { addEventListener: jest.fn(), style: {}, classList: { add: jest.fn(), remove: jest.fn() } };
        this.artistInput = { 
            addEventListener: jest.fn(), 
            value: '', 
            disabled: false,
            setAttribute: jest.fn(),
            setCustomValidity: jest.fn(),
            classList: { add: jest.fn(), remove: jest.fn() },
            validity: { valid: true },
            reportValidity: jest.fn()
        };
        this.playlistSelect = { 
            addEventListener: jest.fn(), 
            value: '', 
            disabled: false,
            innerHTML: '',
            appendChild: jest.fn(),
            selectedOptions: [],
            options: [],
            dispatchEvent: jest.fn(),
            setCustomValidity: jest.fn(),
            classList: { add: jest.fn(), remove: jest.fn() },
            validity: { valid: true },
            reportValidity: jest.fn()
        };
        this.addButton = { 
            addEventListener: jest.fn(), 
            disabled: false,
            classList: { add: jest.fn(), remove: jest.fn() }
        };
        this.overrideButton = { 
            addEventListener: jest.fn(), 
            style: { display: 'none' },
            disabled: false
        };
        this.messageArea = { 
            textContent: '', 
            className: '', 
            style: { display: 'none' },
            setAttribute: jest.fn(),
            scrollIntoView: jest.fn()
        };
        this.playerArea = { innerHTML: '', appendChild: jest.fn(), style: {} };
        
        // Scraping form elements
        this.scrapeForm = { 
            addEventListener: jest.fn(), 
            style: {}, 
            classList: { add: jest.fn(), remove: jest.fn() } 
        };
        this.scrapeUrlInput = { 
            addEventListener: jest.fn(), 
            value: '', 
            disabled: false,
            setCustomValidity: jest.fn(),
            classList: { add: jest.fn(), remove: jest.fn() },
            validity: { valid: true },
            reportValidity: jest.fn()
        };
        this.cssSelectorInput = { 
            addEventListener: jest.fn(), 
            value: '', 
            disabled: false
        };
        this.scrapePlaylistSelect = { 
            addEventListener: jest.fn(), 
            value: '', 
            disabled: false,
            innerHTML: '',
            appendChild: jest.fn(),
            selectedOptions: [],
            options: [],
            setCustomValidity: jest.fn(),
            classList: { add: jest.fn(), remove: jest.fn() },
            validity: { valid: true },
            reportValidity: jest.fn()
        };
        this.scrapeButton = { 
            addEventListener: jest.fn(), 
            disabled: false,
            classList: { add: jest.fn(), remove: jest.fn() }
        };
        this.scrapeMessageArea = { 
            textContent: '', 
            className: '', 
            style: { display: 'none' },
            setAttribute: jest.fn(),
            scrollIntoView: jest.fn()
        };
        this.scrapeResults = { 
            innerHTML: '', 
            classList: { 
                add: jest.fn(), 
                remove: jest.fn(), 
                contains: jest.fn(() => false) 
            },
            appendChild: jest.fn(),
            querySelector: jest.fn(),
            scrollIntoView: jest.fn()
        };
    }

    init() {}
    setupEventListeners() {}
    setupFormValidation() {}
    setupKeyboardNavigation() {}
    setupDeviceOptimizations() {}
    
    async fetchCSRFToken() {
        // Mock implementation
        return Promise.resolve();
    }
    
    async checkAuthStatus() { 
        return Promise.resolve(); 
    }
    
    async loadPlaylists() { 
        return Promise.resolve(); 
    }
    
    populatePlaylistSelect(playlists) {
        this.isUpdatingDropdown = true;
        this.playlistSelect.disabled = playlists.length === 0;
        this.scrapePlaylistSelect.disabled = playlists.length === 0;
        
        // Mock options for both selects
        const mockOptions = [{ value: '', textContent: 'Select a playlist...' }];
        playlists.forEach(playlist => {
            mockOptions.push({
                value: playlist.id,
                textContent: `${playlist.name} (${playlist.track_count} tracks)`,
                dataset: { embedUrl: playlist.embed_url, name: playlist.name.toLowerCase() }
            });
        });
        
        this.playlistSelect.options = mockOptions;
        this.scrapePlaylistSelect.options = mockOptions;
        
        setTimeout(() => { this.isUpdatingDropdown = false; }, 100);
    }
    
    filterPlaylists(searchTerm) {
        if (!searchTerm.trim()) {
            this.filteredPlaylists = [...this.playlists];
        } else {
            this.filteredPlaylists = this.playlists.filter(playlist =>
                playlist.name.toLowerCase().includes(searchTerm.toLowerCase())
            );
        }
    }
    
    validateForm() {
        const artistName = this.artistInput.value.trim();
        const playlistId = this.playlistSelect.value;
        
        if (!artistName) return false;
        if (artistName.length > 100) return false;
        if (!playlistId) return false;
        
        return true;
    }
    
    extractTrackCount(optionText) {
        const match = optionText.match(/\((\d+) tracks?\)/);
        return match ? parseInt(match[1], 10) : 0;
    }
    
    generateEmbedURL(playlistURI) {
        if (playlistURI && playlistURI.startsWith('spotify:playlist:')) {
            const playlistID = playlistURI.replace('spotify:playlist:', '');
            return `https://open.spotify.com/embed/playlist/${playlistID}?utm_source=generator&theme=0`;
        }
        return null;
    }
    
    updatePlayer() {}
    
    showMessage(message, type) {
        this.messageArea.textContent = message;
        this.messageArea.className = `message-area ${type}`;
        this.messageArea.style.display = 'block';
    }
    
    hideMessage() {
        this.messageArea.style.display = 'none';
        this.messageArea.textContent = '';
        this.messageArea.className = 'message-area';
    }
    
    setLoading(loading) {
        this.isLoading = loading;
        this.artistInput.disabled = loading;
        this.playlistSelect.disabled = loading || this.playlists.length === 0;
        this.addButton.disabled = loading;
        this.overrideButton.disabled = loading;
    }
    
    showFieldError(field, message) {
        field.setCustomValidity(message);
        field.classList.add('error');
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
    }
    
    testPlayerCompatibility() {
        const userAgent = navigator.userAgent || '';
        const isMobile = /Android|webOS|iPhone|iPad|iPod|BlackBerry|IEMobile|Opera Mini/i.test(userAgent);
        const isTablet = /iPad|Android(?=.*Mobile)/i.test(userAgent);
        const isDesktop = !isMobile && !isTablet;
        
        return { isMobile, isTablet, isDesktop, compatible: true };
    }
    
    // Scraping methods
    validateScrapeForm() {
        const url = this.scrapeUrlInput.value.trim();
        const playlistId = this.scrapePlaylistSelect.value;
        
        if (!url) return false;
        if (!this.isValidUrl(url)) return false;
        if (!playlistId) return false;
        
        return true;
    }
    
    isValidUrl(string) {
        try {
            const url = new URL(string);
            return url.protocol === 'http:' || url.protocol === 'https:';
        } catch (_) {
            return false;
        }
    }
    
    setScrapeLoading(loading) {
        this.isScraping = loading;
        this.scrapeUrlInput.disabled = loading;
        this.cssSelectorInput.disabled = loading;
        this.scrapePlaylistSelect.disabled = loading || this.playlists.length === 0;
        this.scrapeButton.disabled = loading;
    }
    
    showScrapeMessage(message, type) {
        this.scrapeMessageArea.textContent = message;
        this.scrapeMessageArea.className = `message-area ${type}`;
        this.scrapeMessageArea.style.display = 'block';
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
    
    createStatElement(label, value, type) {
        const stat = document.createElement('div');
        stat.className = 'scrape-stat';
        
        const statLabel = document.createElement('div');
        statLabel.className = 'scrape-stat-label';
        statLabel.textContent = label;
        
        const statValue = document.createElement('div');
        statValue.className = `scrape-stat-value ${type}`;
        statValue.textContent = value.toString();
        
        stat.appendChild(statLabel);
        stat.appendChild(statValue);
        
        return stat;
    }
    
    createArtistResultElement(match) {
        const result = document.createElement('div');
        
        let status = 'error';
        if (match.was_duplicate) {
            status = 'duplicate';
        } else if (match.matched && match.tracks_added > 0) {
            status = 'success';
        }
        
        result.className = `artist-result ${status}`;
        
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
        result.appendChild(content);
        
        return result;
    }
};

// Mock UIUtils
global.UIUtils = {
    debounce: (func, wait) => {
        let timeout;
        return function executedFunction(...args) {
            const later = () => {
                clearTimeout(timeout);
                func(...args);
            };
            clearTimeout(timeout);
            timeout = setTimeout(later, wait);
        };
    },
    
    throttle: (func, limit) => {
        let inThrottle;
        return function() {
            const args = arguments;
            const context = this;
            if (!inThrottle) {
                func.apply(context, args);
                inThrottle = true;
                setTimeout(() => inThrottle = false, limit);
            }
        };
    },
    
    sanitizeHTML: (str) => {
        const temp = document.createElement('div');
        temp.textContent = str;
        return temp.innerHTML;
    }
};

// Mock ErrorHandler
global.ErrorHandler = {
    log: (error, context = '') => {
        const prefix = context ? `[GoListen - ${context}]:` : '[GoListen]:';
        console.error(prefix);
        console.error(error);
    },
    
    handleNetworkError: (error) => {
        if (error.name === 'TypeError' && error.message.includes('Failed to fetch')) {
            return 'Network error. Please check your connection and try again.';
        }
        return error.message;
    }
};

// Reset mocks before each test
beforeEach(() => {
    jest.clearAllMocks();
    fetch.mockClear();
});