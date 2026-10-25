'use strict';

const TrailApp = (() => {
    const progressKey = 'trails-completionist:progress:v1';
    const preferencesKey = 'trails-completionist:preferences:v1';
    const columns = {
        'trail name': 'name', name: 'name', 'park name': 'park', park: 'park',
        'trail type': 'type', type: 'type', 'trail length': 'length', length: 'length',
        'date completed': 'date', completed: 'completed', source: 'source',
        'osm match': 'match', status: 'status', 'record id': 'id', id: 'id',
    };
    const fieldPattern = new RegExp('\\b(' + Object.keys(columns).join('|') + ')\\s*:', 'gi');
    const collator = new Intl.Collator(undefined, { numeric: true, sensitivity: 'base' });
    const numbers = new Intl.NumberFormat();
    const normalize = value => String(value ?? '').normalize('NFD').replace(/[\u0300-\u036f]/g, '').toLowerCase();
    const booleanValue = value => {
        const text = normalize(value).trim();
        if (['yes', 'true', 'completed'].includes(text)) return true;
        if (['no', 'false', 'incomplete', 'remaining'].includes(text)) return false;
        return null;
    };

    function parseQuery(query) {
        const quoted = new Set();
        let quote = '';
        for (let index = 0; index < query.length; index++) {
            if (quote) quoted.add(index);
            if (query[index] === '\\') { if (quote) quoted.add(++index); }
            else if (quote === query[index]) quote = '';
            else if (!quote && (query[index] === '"' || query[index] === "'")) quote = query[index];
        }
        const matches = Array.from(query.matchAll(new RegExp(fieldPattern))).filter(match => !quoted.has(match.index));
        const filters = [];
        let freeText = matches.length ? query.slice(0, matches[0].index) : query;
        matches.forEach((match, index) => {
            let value = query.slice(match.index + match[0].length, matches[index + 1]?.index ?? query.length).trim();
            const column = columns[normalize(match[1])];
            if (value[0] === '"' || value[0] === "'") {
                const end = value.indexOf(value[0], 1);
                if (end > 0) {
                    freeText += ' ' + value.slice(end + 1);
                    value = value.slice(1, end);
                }
            } else if (['completed', 'date', 'length'].includes(column)) {
                const tokens = value.split(/\s+/);
                value = tokens.shift() || '';
                freeText += ' ' + tokens.join(' ');
            }
            filters.push({ column, value: normalize(value) });
        });
        return { filters, words: normalize(freeText).trim().split(/\s+/).filter(Boolean) };
    }

    function fuzzyMatch(term, text) {
        let position = 0;
        for (const char of term) {
            position = text.indexOf(char, position);
            if (position === -1) return false;
            position++;
        }
        return true;
    }

    function distanceValue(value) {
        if (value === null || value === undefined || String(value).trim() === '') return null;
        const number = Number(String(value).replace(/\s*(mi|miles)\s*$/i, ''));
        return Number.isFinite(number) && number >= 0 ? number : null;
    }

    function isoDate(value) {
        const text = String(value || '');
        const match = text.match(/^(\d{4})-(\d{2})-(\d{2})$/) || text.match(/^(\d{1,2})\/(\d{1,2})\/(\d{4})$/);
        if (!match) return '';
        const [year, month, day] = text.includes('/') ? [Number(match[3]), Number(match[1]), Number(match[2])] : match.slice(1).map(Number);
        const date = new Date(Date.UTC(year, month - 1, day));
        if (date.getUTCFullYear() !== year || date.getUTCMonth() !== month - 1 || date.getUTCDate() !== day) return '';
        return `${year}-${String(month).padStart(2, '0')}-${String(day).padStart(2, '0')}`;
    }

    function displayDate(value) {
        const iso = isoDate(value);
        return iso ? `${iso.slice(5, 7)}/${iso.slice(8, 10)}/${iso.slice(0, 4)}` : '';
    }

    function today() {
        const date = new Date();
        return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`;
    }

    function refreshSearch(record) {
        record.search = normalize([
            record.name, record.label, record.reference, record.park, record.type, record.length === '' ? '' : `${record.length} miles`,
            record.sourceName, record.sourceValue, record.status, record.match, record.id, record.date,
        ].join(' '));
    }

    function prepareRecords(data, overrides = new Map()) {
        const occurrences = new Map();
        return data.records.map((item, key) => {
            const source = data.sources[item.source] || { name: 'Unspecified source', value: '' };
            const fallback = JSON.stringify([item.name, item.park, item.type, item.length, source.value]);
            const occurrence = occurrences.get(fallback) || 0;
            occurrences.set(fallback, occurrence + 1);
            const record = {
                ...item, key, name: item.name || 'Unnamed trail', park: item.park || '', type: item.type || '',
                length: item.length ?? '', date: displayDate(item.date), completed: Boolean(item.completed),
                label: item.label || item.name || 'Unnamed trail',
                sourceName: source.name, sourceValue: source.value, sourceURL: source.url || '',
                status: item.status || '', match: item.match || (item.incomplete ? 'incomplete' : ''),
                distance: distanceValue(item.length), storageID: item.id || `local/${fallback}/${occurrence}`,
            };
            record.baseCompleted = record.completed;
            record.baseDate = record.date;
            record.primaryStatus = record.status.split(',')[0].trim();
            const saved = overrides.get(record.storageID);
            // Fresh GPX imports take precedence over overrides from an older snapshot.
            if (saved && typeof saved.completed === 'boolean' && saved.baseCompleted === record.baseCompleted && saved.baseDate === record.baseDate) {
                record.completed = saved.completed;
                record.date = saved.completed ? displayDate(saved.date) : '';
            }
            refreshSearch(record);
            return record;
        });
    }

    function matchesRecord(record, state, query) {
        if (state.completion === 'completed' && !record.completed) return false;
        if (state.completion === 'remaining' && record.completed) return false;
        if (state.park && (record.park || '__unspecified__') !== state.park) return false;
        if (state.type && (record.type || '__unspecified__') !== state.type) return false;
        if (state.source && (record.sourceValue || '__unspecified__') !== state.source) return false;
        if (state.status && (record.primaryStatus || '__unspecified__') !== state.status) return false;
        if (state.match && (record.match || '__unspecified__') !== state.match) return false;
        const minimum = distanceValue(state.min);
        const maximum = distanceValue(state.max);
        if (state.invalidDistance || [state.min, state.max].some(value => value !== '' && value != null && distanceValue(value) === null)) return false;
        if (minimum !== null && maximum !== null && minimum > maximum) return false;
        if (minimum !== null && (record.distance === null || record.distance < minimum)) return false;
        if (maximum !== null && (record.distance === null || record.distance > maximum)) return false;
        for (const filter of query.filters) {
            if (filter.column === 'completed') {
                const expected = booleanValue(filter.value);
                if (expected === null || record.completed !== expected) return false;
            } else {
                const value = filter.column === 'source' ? `${record.sourceName} ${record.sourceValue}` : record[filter.column];
                if (!normalize(value).includes(filter.value)) return false;
            }
        }
        return query.words.every(word => {
            const expected = booleanValue(word);
            return expected === null ? fuzzyMatch(word, record.search) : record.completed === expected;
        });
    }

    function sortRecords(records, order) {
        return records.slice().sort((a, b) => {
            if (order === 'shortest' || order === 'longest') {
                if (a.distance === null && b.distance !== null) return 1;
                if (b.distance === null && a.distance !== null) return -1;
                const difference = (a.distance ?? 0) - (b.distance ?? 0);
                if (difference) return order === 'shortest' ? difference : -difference;
            } else if (order === 'recent') {
                if (a.completed !== b.completed) return a.completed ? -1 : 1;
                const difference = isoDate(b.date).localeCompare(isoDate(a.date));
                if (difference) return difference;
            } else if (order === 'park') {
                const difference = collator.compare(a.park || '\uffff', b.park || '\uffff');
                if (difference) return difference;
            }
            return collator.compare(a.name, b.name) || a.key - b.key;
        });
    }

    function csvText(records) {
        const cell = value => {
            let text = String(value ?? '');
            if (/^[\s]*[=+\-@]/.test(text)) text = "'" + text;
            return '"' + text.replace(/"/g, '""') + '"';
        };
        const rows = [['Record ID', 'Trail name', 'Park or area', 'Type', 'Length (miles)', 'Completed', 'Completion date', 'Source', 'Source status', 'OSM match', 'URL']];
        records.forEach(record => rows.push([record.id, record.name, record.park, record.type, record.length, record.completed ? 'Yes' : 'No', record.date, record.sourceValue, record.status, record.match, record.url]));
        return '\ufeff' + rows.map(row => row.map(cell).join(',')).join('\r\n') + '\r\n';
    }

    function statusNote(record) {
        const notes = { proposed: 'Proposed', removed: 'Removed', closed: 'Closed', decommissioned: 'Decommissioned',
            'under construction': 'Under construction', 'restricted private': 'Restricted / private', 'open fee': 'Entry fee' };
        return notes[normalize(record.primaryStatus).replace(/_/g, ' ')] || '';
    }

    function mount(doc) {
        const payload = doc.getElementById('trail-data');
        if (!payload) return;
        const data = JSON.parse(payload.textContent);
        const win = doc.defaultView;
        const get = id => doc.getElementById(id);
        const readStorage = key => {
            try {
                const value = JSON.parse(win.localStorage.getItem(key) || '{}');
                return value && typeof value === 'object' && !Array.isArray(value) ? value : {};
            } catch { return {}; }
        };
        const overrides = new Map(Object.entries(readStorage(progressKey)));
        const preferences = readStorage(preferencesKey);
        const records = prepareRecords(data, overrides);
        const media = win.matchMedia('(max-width: 760px)');
        const params = new URL(win.location.href).searchParams;
        const choices = ['park', 'type', 'source', 'status', 'match'];
        const state = {
            query: params.get('q') || '', completion: ['completed', 'remaining'].includes(params.get('completion')) ? params.get('completion') : 'all',
            sort: ['name', 'park', 'shortest', 'longest', 'recent'].includes(params.get('sort')) ? params.get('sort') : 'name',
            min: params.get('min') || '', max: params.get('max') || '',
            view: ['list', 'cards'].includes(params.get('view')) ? params.get('view') : (['list', 'cards'].includes(preferences.view) ? preferences.view : 'auto'),
            page: Math.max(1, Number(params.get('page')) || 1), size: [24, 48, 96].includes(Number(params.get('size'))) ? Number(params.get('size')) : 24,
        };
        let filtered = [];
        let activeRecord = null;
        let toastTimer;
        let searchTimer;
        let lastDialogTrigger = null;
        const create = (tag, className, text) => {
            const element = doc.createElement(tag);
            if (className) element.className = className;
            if (text !== undefined) element.textContent = text;
            return element;
        };
        const icon = name => {
            const svg = doc.createElementNS('http://www.w3.org/2000/svg', 'svg');
            svg.setAttribute('class', 'icon');
            svg.setAttribute('aria-hidden', 'true');
            const use = doc.createElementNS('http://www.w3.org/2000/svg', 'use');
            use.setAttribute('href', `#i-${name}`);
            svg.append(use);
            return svg;
        };
        const showToast = message => {
            win.clearTimeout(toastTimer);
            get('toast').textContent = message;
            get('toast').hidden = false;
            toastTimer = win.setTimeout(() => { get('toast').hidden = true; }, 5000);
        };
        const savePreferences = () => {
            try { win.localStorage.setItem(preferencesKey, JSON.stringify(preferences)); } catch { /* The current page remains usable without storage. */ }
        };
        const setTheme = value => {
            preferences.theme = ['system', 'light', 'dark'].includes(value) ? value : 'system';
            doc.documentElement.dataset.theme = preferences.theme;
            const next = { system: 'light', light: 'dark', dark: 'system' }[preferences.theme];
            get('theme-button').setAttribute('aria-label', `Color theme: ${preferences.theme}. Select ${next} theme.`);
            get('theme-button').title = `Color theme: ${preferences.theme}`;
            get('theme-label').textContent = `${preferences.theme} theme`;
        };
        setTheme(preferences.theme);
        get('theme-button').addEventListener('click', () => {
            setTheme({ system: 'light', light: 'dark', dark: 'system' }[preferences.theme]);
            savePreferences();
        });
        const populate = (id, values, labelFor = value => value) => {
            const select = get(id);
            Array.from(new Set(values)).sort(collator.compare).forEach(value => {
                const option = create('option', '', labelFor(value));
                option.value = value;
                select.append(option);
            });
        };
        const label = value => value === '__unspecified__' ? 'Unspecified' : value.charAt(0).toUpperCase() + value.slice(1);
        populate('filter-park', records.map(record => record.park || '__unspecified__'), label);
        populate('filter-type', records.map(record => record.type || '__unspecified__'), label);
        populate('filter-status', records.map(record => record.primaryStatus || '__unspecified__'), label);
        populate('filter-match', records.map(record => record.match || '__unspecified__'), value => value === '__unspecified__' ? 'Not audited' : label(value));
        populate('filter-source', data.sources.map(source => source.value || '__unspecified__'), value => data.sources.find(source => source.value === value)?.name || 'Unspecified source');
        choices.forEach(key => {
            const select = get(`filter-${key}`);
            select.value = params.get(key) || '';
            state[key] = select.value;
        });
        get('fuzzySearch').value = state.query;
        get('sort-order').value = state.sort;
        get('length-min').value = state.min;
        get('length-max').value = state.max;
        state.min = get('length-min').value;
        state.max = get('length-max').value;
        get('page-size').value = String(state.size);
        const effectiveView = () => state.view === 'auto' ? (media.matches ? 'cards' : 'list') : state.view;
        const updateURL = () => {
            const url = new URL(win.location.href);
            const values = { q: state.query, completion: state.completion === 'all' ? '' : state.completion, sort: state.sort === 'name' ? '' : state.sort,
                park: state.park, type: state.type, source: state.source, status: state.status, match: state.match, min: state.min, max: state.max,
                view: state.view === 'auto' ? '' : state.view, page: state.page === 1 ? '' : state.page, size: state.size === 24 ? '' : state.size };
            Object.entries(values).forEach(([key, value]) => { if (value !== '') url.searchParams.set(key, value); else url.searchParams.delete(key); });
            try { win.history.replaceState(null, '', url); } catch { /* Some file browsers do not permit history changes. */ }
        };
        const distances = new Intl.NumberFormat(undefined, { maximumFractionDigits: 3 });
        const distanceText = record => record.distance === null ? 'Not recorded' : `${distances.format(record.distance)} mi`;
        const completionControl = record => {
            const control = create('label', 'completion-control');
            const checkbox = create('input');
            checkbox.type = 'checkbox';
            checkbox.checked = record.completed;
            checkbox.dataset.action = 'complete';
            checkbox.dataset.recordKey = String(record.key);
            checkbox.setAttribute('aria-label', `Mark ${record.label}${record.reference ? ', ' + record.reference : ''} ${record.completed ? 'incomplete' : 'complete'}`);
            const text = create('span', '', record.completed ? 'Completed' : 'To explore');
            if (record.completed && record.date) text.append(create('small', '', record.date));
            control.append(checkbox, text);
            return control;
        };
        const detailButton = record => {
            const button = create('button', 'icon-button detail-button');
            button.type = 'button';
            button.dataset.action = 'details';
            button.dataset.recordKey = String(record.key);
            button.setAttribute('aria-label', `Details for ${record.label}${record.reference ? ', ' + record.reference : ''}`);
            button.append(icon('arrow'));
            return button;
        };
        const trailRow = record => {
            const row = create('tr');
            row.dataset.recordKey = String(record.key);
            const title = create('td');
            title.append(create('span', 'trail-title', record.label));
            if (record.reference) title.append(create('span', 'record-reference', record.reference));
            if (record.incomplete) title.append(create('span', 'record-note', 'Incomplete geometry'));
            if (statusNote(record)) title.append(create('span', 'record-note status-note', statusNote(record)));
            const type = create('td');
            type.append(create('span', 'type-tag', record.type || 'Unspecified'));
            const completion = create('td');
            completion.append(completionControl(record));
            const action = create('td');
            action.append(detailButton(record));
            row.append(title, create('td', record.park ? '' : 'muted', record.park || 'Unspecified area'), create('td', 'distance-value', distanceText(record)), type, completion, action);
            return row;
        };
        const trailCard = record => {
            const card = create('article', 'trail-card');
            const topline = create('div', 'card-topline');
            topline.append(create('span', 'type-tag', record.type || 'Unspecified'), create('span', 'card-number', String(record.key + 1).padStart(3, '0')));
            const main = create('div', 'card-main');
            main.append(create('h3', '', record.label));
            if (record.reference) main.append(create('p', 'record-reference', record.reference));
            const park = create('p', 'card-park');
            park.append(icon('pin'), doc.createTextNode(record.park || 'Unspecified area'));
            main.append(park);
            if (record.incomplete) main.append(create('p', 'record-note', 'Incomplete geometry'));
            if (statusNote(record)) main.append(create('p', 'record-note status-note', statusNote(record)));
            const distance = create('p', 'card-distance');
            if (record.distance === null) distance.append(create('span', '', 'Distance not recorded'));
            else {
                distance.append(doc.createTextNode(distanceText(record).replace(/ mi$/, '')));
                distance.append(create('span', '', 'miles'));
            }
            main.append(distance);
            const footer = create('div', 'card-footer');
            footer.append(completionControl(record), detailButton(record));
            card.append(topline, main, footer);
            return card;
        };
        const updateTotals = () => {
            const count = records.filter(record => record.completed).length;
            const percent = records.length ? Math.round(count / records.length * 100) : 0;
            const values = { 'stat-total': records.length, 'stat-completed': count, 'stat-remaining': records.length - count,
                'tab-all': records.length, 'tab-completed': count, 'tab-remaining': records.length - count,
                'progress-completed': count, 'progress-total': records.length, 'progress-percent': percent };
            Object.entries(values).forEach(([id, value]) => { get(id).textContent = numbers.format(value); });
            get('progress-fill').style.width = `${percent}%`;
            get('completion-progress').setAttribute('aria-valuenow', String(percent));
        };
        const render = () => {
            const focused = get('results').contains(doc.activeElement) ? doc.activeElement.dataset.recordKey : undefined;
            filtered = sortRecords(records.filter(record => matchesRecord(record, state, parseQueryCache)), state.sort);
            const pageCount = Math.max(1, Math.ceil(filtered.length / state.size));
            state.page = Math.min(pageCount, Math.max(1, Math.floor(state.page) || 1));
            const start = (state.page - 1) * state.size;
            const page = filtered.slice(start, start + state.size);
            const view = effectiveView();
            const fragment = doc.createDocumentFragment();
            page.forEach(record => fragment.append(view === 'cards' ? trailCard(record) : trailRow(record)));
            get('tableBody').replaceChildren();
            get('cards-view').replaceChildren();
            get(view === 'cards' ? 'cards-view' : 'tableBody').append(fragment);
            get('list-view').hidden = view !== 'list' || !filtered.length;
            get('cards-view').hidden = view !== 'cards' || !filtered.length;
            get('empty-state').hidden = Boolean(filtered.length);
            if (!records.length) {
                get('empty-state').querySelector('h3').textContent = 'No trail records yet.';
                get('empty-state').querySelector('p').textContent = 'Add a trail inventory. Generate this page again to start exploring.';
                get('empty-reset').hidden = true;
            }
            get('pagination').hidden = !filtered.length;
            get('previous-page').disabled = state.page === 1;
            get('next-page').disabled = state.page >= pageCount;
            get('page-number').value = String(state.page);
            get('page-number').max = String(pageCount);
            get('page-total').textContent = `of ${numbers.format(pageCount)}`;
            get('results-count').textContent = filtered.length ? `Showing ${numbers.format(start + 1)}–${numbers.format(start + page.length)} of ${numbers.format(filtered.length)} trail records` : '0 trail records match';
            get('export-button').disabled = !filtered.length;
            get('export-button').title = `Export all ${numbers.format(filtered.length)} matching records`;
            doc.querySelectorAll('[data-completion]').forEach(button => button.setAttribute('aria-pressed', String(button.dataset.completion === state.completion)));
            doc.querySelectorAll('[data-view]').forEach(button => button.setAttribute('aria-pressed', String(button.dataset.view === view)));
            const count = choices.filter(key => state[key]).length + Number(state.min !== '') + Number(state.max !== '');
            get('filter-count').textContent = String(count);
            get('filter-count').hidden = !count;
            const invalidValue = state.invalidDistance || [state.min, state.max].some(value => value !== '' && distanceValue(value) === null);
            const invalidRange = invalidValue || (distanceValue(state.min) !== null && distanceValue(state.max) !== null && Number(state.min) > Number(state.max));
            get('distance-hint').textContent = invalidValue ? 'Enter distances of zero or more.' : (invalidRange ? 'Minimum distance must not exceed maximum distance.' : 'Filters apply to the full collection.');
            get('distance-hint').classList.toggle('invalid', invalidRange);
            get('length-min').setAttribute('aria-invalid', String(invalidRange));
            get('length-max').setAttribute('aria-invalid', String(invalidRange));
            updateTotals();
            updateURL();
            if (focused !== undefined) {
                const replacement = get('results').querySelector(`input[data-record-key="${focused}"]`);
                (replacement || get('collection-title')).focus({ preventScroll: true });
            }
        };
        let parseQueryCache = parseQuery(state.query);
        const applyFilters = () => {
            state.query = get('fuzzySearch').value;
            state.min = get('length-min').value;
            state.max = get('length-max').value;
            state.invalidDistance = get('length-min').validity.badInput || get('length-max').validity.badInput;
            choices.forEach(key => { state[key] = get(`filter-${key}`).value; });
            state.page = 1;
            parseQueryCache = parseQuery(state.query);
            render();
        };
        get('fuzzySearch').addEventListener('input', () => { win.clearTimeout(searchTimer); searchTimer = win.setTimeout(applyFilters, 120); });
        choices.forEach(key => get(`filter-${key}`).addEventListener('change', applyFilters));
        ['length-min', 'length-max'].forEach(id => get(id).addEventListener('input', applyFilters));
        get('sort-order').addEventListener('change', () => { state.sort = get('sort-order').value; state.page = 1; render(); });
        doc.querySelectorAll('[data-completion]').forEach(button => button.addEventListener('click', () => { state.completion = button.dataset.completion; state.page = 1; render(); }));
        doc.querySelectorAll('[data-view]').forEach(button => button.addEventListener('click', () => { state.view = button.dataset.view; preferences.view = state.view; savePreferences(); render(); }));
        get('filters-toggle').addEventListener('click', () => {
            const open = get('filter-panel').hidden;
            get('filter-panel').hidden = !open;
            get('filters-toggle').setAttribute('aria-expanded', String(open));
        });
        if (choices.some(key => state[key]) || state.min || state.max) {
            get('filter-panel').hidden = false;
            get('filters-toggle').setAttribute('aria-expanded', 'true');
        }
        const reset = () => {
            get('fuzzySearch').value = '';
            get('length-min').value = '';
            get('length-max').value = '';
            choices.forEach(key => { get(`filter-${key}`).value = ''; });
            state.completion = 'all';
            applyFilters();
        };
        get('reset-filters').addEventListener('click', reset);
        get('empty-reset').addEventListener('click', reset);
        get('page-size').addEventListener('change', () => { state.size = Number(get('page-size').value); state.page = 1; render(); });
        get('page-number').addEventListener('change', () => { state.page = Number(get('page-number').value); render(); });
        ['previous-page', 'next-page'].forEach(id => get(id).addEventListener('click', () => {
            state.page += id === 'next-page' ? 1 : -1;
            render();
            get('collection-title').focus({ preventScroll: true });
        }));
        media.addEventListener('change', () => { if (state.view === 'auto') render(); });
        const safeLink = (value, text) => {
            try {
                const parsed = new URL(value);
                if (!['https:', 'http:'].includes(parsed.protocol)) return null;
                const link = create('a');
                link.href = parsed.href;
                link.target = '_blank';
                link.rel = 'noopener noreferrer';
                link.append(doc.createTextNode(text), icon('link'));
                return link;
            } catch { return null; }
        };
        const renderDetails = () => {
            const record = activeRecord;
            if (!record) return;
            const keepCheckboxFocus = get('detail-completion').contains(doc.activeElement);
            get('detail-title').textContent = record.label;
            get('detail-park').textContent = record.park || 'Unspecified area';
            get('detail-completion').replaceChildren(completionControl(record));
            get('completion-date').value = record.completed ? isoDate(record.date) : '';
            get('completion-date').max = today();
            get('completion-date').disabled = !record.completed;
            get('detail-id').textContent = record.id || 'No source ID';
            const facts = [ ['Distance', distanceText(record)], ['Type', record.type || 'Unspecified'], ['Source', record.sourceName],
                ['Source status', record.status || 'Not specified'], ['OSM match', record.match || 'Not audited'],
                ['Geometry', record.incomplete ? 'Incomplete' : (record.mapURL ? 'Location available' : 'Location not included')] ];
            if (record.label !== record.name) facts.push(['Source name', record.name]);
            get('detail-facts').replaceChildren(...facts.map(([name, value]) => {
                const row = create('div');
                row.append(create('dt', '', name), create('dd', '', value));
                return row;
            }));
            get('detail-links').replaceChildren();
            [[record.url, 'Trail website'], [record.mapURL, 'View on OpenStreetMap'], [record.sourceURL, 'View source']].forEach(([value, text]) => {
                if (value) { const link = safeLink(value, text); if (link) get('detail-links').append(link); }
            });
            if (keepCheckboxFocus) get('detail-completion').querySelector('input').focus({ preventScroll: true });
        };
        const saveProgress = record => {
            overrides.set(record.storageID, { baseCompleted: record.baseCompleted, baseDate: record.baseDate, completed: record.completed, date: record.date });
            let saved = true;
            try { win.localStorage.setItem(progressKey, JSON.stringify(Object.fromEntries(overrides))); } catch { saved = false; }
            refreshSearch(record);
            render();
            renderDetails();
            showToast(saved ? (record.completed ? 'Trail marked complete. Progress saved in this browser.' : 'Trail marked to explore. Progress saved in this browser.') : 'Progress updated for this session. Browser storage is unavailable. Export CSV to keep your changes.');
        };
        doc.addEventListener('change', event => {
            const target = event.target;
            if (target.dataset.action !== 'complete') return;
            const record = records[Number(target.dataset.recordKey)];
            if (!record) return;
            record.completed = target.checked;
            record.date = record.completed ? displayDate(today()) : '';
            saveProgress(record);
        });
        get('results').addEventListener('click', event => {
            const button = event.target.closest('[data-action="details"]');
            if (!button) return;
            activeRecord = records[Number(button.dataset.recordKey)];
            lastDialogTrigger = button;
            renderDetails();
            get('trail-dialog').showModal();
            get('close-dialog').focus();
        });
        get('completion-date').addEventListener('change', () => {
            if (!activeRecord || !activeRecord.completed) return;
            if (!get('completion-date').reportValidity()) return;
            activeRecord.date = displayDate(get('completion-date').value);
            saveProgress(activeRecord);
        });
        get('close-dialog').addEventListener('click', () => get('trail-dialog').close());
        get('trail-dialog').addEventListener('click', event => { if (event.target === get('trail-dialog')) get('trail-dialog').close(); });
        get('trail-dialog').addEventListener('close', () => {
            const key = activeRecord?.key;
            activeRecord = null;
            const trigger = lastDialogTrigger?.isConnected ? lastDialogTrigger : get('results').querySelector(`button[data-record-key="${key}"]`);
            (trigger || get('collection-title')).focus({ preventScroll: true });
        });
        get('export-button').addEventListener('click', () => {
            const url = win.URL.createObjectURL(new Blob([csvText(filtered)], { type: 'text/csv;charset=utf-8' }));
            const link = create('a');
            link.href = url;
            link.download = `trails-${today()}.csv`;
            doc.body.append(link);
            link.click();
            link.remove();
            win.setTimeout(() => win.URL.revokeObjectURL(url), 1000);
            showToast(`Exported ${numbers.format(filtered.length)} matching records.`);
        });
        doc.addEventListener('keydown', event => {
            const editing = event.target.closest?.('input, select, textarea, [contenteditable]');
            if (event.key === '/' && !editing && !get('trail-dialog').open && !event.ctrlKey && !event.metaKey && !event.altKey) {
                event.preventDefault();
                get('fuzzySearch').focus();
            }
            if (event.key === 'Escape' && doc.activeElement === get('fuzzySearch')) { get('fuzzySearch').value = ''; applyFilters(); }
        });
        render();
    }

    return { parseQuery, fuzzyMatch, matchesRecord, prepareRecords, sortRecords, distanceValue, isoDate, csvText, mount };
})();

if (typeof module !== 'undefined' && module.exports) module.exports = TrailApp;
if (typeof document !== 'undefined') document.addEventListener('DOMContentLoaded', () => TrailApp.mount(document));
