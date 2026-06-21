/**
 * app.js - Advanced Search and Filtering for trails-completionist HTML table
 * 
 * This script provides flexible search functionality 
 * that supports:
 * - Fuzzy text matching
 * - Specific column filtering
 * - Combining filters and free text search
 */

document.addEventListener('DOMContentLoaded', () => {
    // Select key DOM elements
    const tableBody = document.getElementById('tableBody');
    const fuzzySearch = document.getElementById('fuzzySearch');
    const rows = Array.from(tableBody.querySelectorAll('tr'));
    
    /**
     * Mapping of user-friendly column names to data attributes
     * Allows more natural search query parsing
     */
    const columnMapping = {
        'trail name': 'trailName',
        'park name': 'parkName',
        'trail type': 'trailType',
        'trail length': 'trailLength',
        'completed': 'completed',
        'date completed': 'dateCompleted'
    };

    /**
     * Parses complex search queries into specific filters and free text
     * 
     * @param {string} query - The search query string
     * @returns {Object} An object with parsed filters and remaining free text
     */
    const parseSearchQuery = (query) => {
        const filters = {};
        let freeText = query;

        // Regex to extract column:value pairs
        const filterRegex = /(\w+\s*\w*)\s*:\s*([^:]+)(?=\s+\w+:|$)/g;
        let match;

        while ((match = filterRegex.exec(query)) !== null) {
            const columnName = match[1].trim().toLowerCase();
            const value = match[2].trim();
            
            // Map user-friendly column names to data attributes
            const mappedColumn = columnMapping[columnName];
            if (mappedColumn) {
                filters[mappedColumn] = value;
                // Remove the matched filter from free text
                freeText = freeText.replace(match[0], '').trim();
            }
        }

        return { filters, freeText };
    };

    /**
     * Performs fuzzy matching of search characters
     * 
     * @param {string} searchTerm - The search term to match
     * @param {string} text - The text to search within
     * @returns {boolean} Whether the search term matches the text
     */
    const fuzzyMatch = (searchTerm, text) => {
        // Case-insensitive matching
        searchTerm = searchTerm.toLowerCase();
        text = text.toLowerCase();

        // Check if characters appear in order
        const searchChars = searchTerm.split('');
        let lastIndex = -1;

        for (let char of searchChars) {
            lastIndex = text.indexOf(char, lastIndex + 1);
            if (lastIndex === -1) return false;
        }

        return true;
    };

    /**
     * Performs the search and filtering on the table
     */
    const performSearch = () => {
        const searchTerm = fuzzySearch.value.trim();
        const { filters, freeText } = parseSearchQuery(searchTerm);

        rows.forEach(row => {
            // Check specific filters first
            let matchesFilters = true;
            for (const [column, value] of Object.entries(filters)) {
                const columnHeader = document.querySelector(`th[data-column="${column}"]`);
                const columnIndex = Array.from(columnHeader.parentNode.children).indexOf(columnHeader);
                const cell = row.querySelector(`td:nth-child(${columnIndex + 1})`);
                const cellText = cell.textContent.trim();
                
                // Special handling for 'completed' column
                if (column === 'completed' && (value.toLowerCase() === 'true' || value.toLowerCase() === 'yes')) {
                    const checkbox = cell.querySelector('input[type="checkbox"]');
                    if (!checkbox || !checkbox.checked) {
                        matchesFilters = false;
                        break;
                    }
                } else if (!cellText.toLowerCase().includes(value.toLowerCase())) {
                    matchesFilters = false;
                    break;
                }
            }

            // Apply fuzzy search to remaining free text
            const cellTexts = Array.from(row.querySelectorAll('td'))
                .map(cell => cell.textContent.trim());
            
            const freeTextMatch = freeText === '' || 
                cellTexts.some(text => fuzzyMatch(freeText, text));

            // Show row only if both filter and free text conditions are met
            row.style.display = (matchesFilters && freeTextMatch) ? '' : 'none';
        });
    };

    // Add event listener for real-time search
    fuzzySearch.addEventListener('input', performSearch);
});