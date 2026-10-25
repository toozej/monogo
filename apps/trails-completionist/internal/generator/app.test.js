const test = require('node:test');
const assert = require('node:assert/strict');
const app = require('./app.js');

const data = {
    sources: [{ name: 'Portland Parks', value: 'https://example.test/parks' }, { name: 'Regional trails', value: 'regional' }],
    records: [
        { id: 'city/1', name: 'Maple Trail', park: 'Forest Park', type: 'Moderate', length: '2.5', completed: true, date: '07/10/2026', source: 0, status: 'existing, UserPedestrian=yes', match: 'geometry' },
        { id: 'city/2', name: 'Cedar Trail', park: 'Forest Park', type: 'Moderate', length: '10', completed: false, source: 0, status: 'proposed', match: 'missing' },
        { id: 'regional/1', name: 'Maple Loop', park: 'Another Park', type: 'Connector', length: '0', completed: false, source: 1, status: 'Open' },
        { id: 'city/3', name: 'Unknown distance', park: '', type: '', length: '', completed: false, source: 0, incomplete: true },
    ],
};
const records = () => app.prepareRecords(data);
const all = { completion: 'all', min: '', max: '' };
const matching = (query, state = all) => records().filter(record => app.matchesRecord(record, state, app.parseQuery(query))).map(record => record.id);

test('search spans fields and retains completion shortcuts', () => {
    assert.deepEqual(matching('Moderate yes'), ['city/1']);
    assert.deepEqual(matching('completed:no'), ['city/2', 'regional/1', 'city/3']);
    assert.deepEqual(matching('park name: Forest Park completed: yes'), ['city/1']);
    assert.deepEqual(matching('maple park name:"Forest Park" completed:yes'), ['city/1']);
    assert.deepEqual(matching('osm match: missing source: Portland Parks'), ['city/2']);
    assert.deepEqual(matching('record id:city/1'), ['city/1']);
    assert.deepEqual(matching('source:https://example.test/parks completed:yes'), ['city/1']);
    assert.deepEqual(matching('completed:maybe'), []);
});

test('filters combine park, type, source, status, match, completion, and numeric distance', () => {
    assert.deepEqual(matching('', { ...all, park: 'Forest Park', type: 'Moderate', source: 'https://example.test/parks', status: 'existing', match: 'geometry', min: '2', max: '3' }), ['city/1']);
    assert.deepEqual(matching('', { ...all, min: '0', max: '0' }), ['regional/1']);
    assert.deepEqual(matching('', { ...all, min: '4', max: '2' }), []);
    assert.deepEqual(matching('', { ...all, min: '-1' }), []);
    assert.deepEqual(matching('', { ...all, invalidDistance: true }), []);
    assert.deepEqual(matching('', { ...all, park: '__unspecified__', match: 'incomplete' }), ['city/3']);
    assert.deepEqual(matching('', { ...all, completion: 'remaining', park: 'Forest Park' }), ['city/2']);
});

test('distance sorting uses numbers and places missing distances last', () => {
    assert.deepEqual(app.sortRecords(records(), 'shortest').map(record => record.id), ['regional/1', 'city/1', 'city/2', 'city/3']);
    assert.deepEqual(app.sortRecords(records(), 'longest').map(record => record.id), ['city/2', 'city/1', 'regional/1', 'city/3']);
    assert.equal(app.sortRecords(records(), 'recent')[0].id, 'city/1');
    assert.equal(app.distanceValue(''), null);
    assert.equal(app.distanceValue('NaN'), null);
    assert.equal(app.distanceValue('0.0 miles'), 0);
});

test('manual changes use stable source IDs and do not erase fresh GPX completions', () => {
    const overrides = new Map([['city/2', { baseCompleted: false, baseDate: '', completed: true, date: '08/11/2026' }]]);
    const changed = app.prepareRecords(data, overrides);
    assert.equal(changed[1].completed, true);
    assert.equal(changed[1].date, '08/11/2026');
    const refreshed = structuredClone(data);
    refreshed.records[1].completed = true;
    refreshed.records[1].date = '09/01/2026';
    const imported = app.prepareRecords(refreshed, overrides);
    assert.equal(imported[1].completed, true);
    assert.equal(imported[1].date, '09/01/2026');
    assert.equal(changed[2].completed, false);
});

test('CSV retains metadata and dates and neutralizes spreadsheet formulas', () => {
    const rows = records();
    rows[0].name = '=HYPERLINK("bad")';
    const csv = app.csvText(rows);
    assert.ok(csv.startsWith('\ufeff"Record ID"'));
    assert.ok(csv.includes('"\'=HYPERLINK(""bad"")"'));
    assert.ok(csv.includes('"07/10/2026"'));
    assert.ok(csv.includes('"https://example.test/parks"'));
    assert.ok(csv.includes('"existing, UserPedestrian=yes"'));
    assert.equal(csv.split('\r\n').length, 6);
});

test('date parsing rejects invalid dates without shifting calendar days', () => {
    assert.equal(app.isoDate('7/5/2026'), '2026-07-05');
    assert.equal(app.isoDate('2024-02-29'), '2024-02-29');
    assert.equal(app.isoDate('02/29/2026'), '');
    assert.equal(app.isoDate('not a date'), '');
});

test('quoted values can contain field-like text without creating another filter', () => {
    const query = app.parseQuery('name:"Trail park: connector" completed:no');
    assert.deepEqual(query.filters, [{ column: 'name', value: 'trail park: connector' }, { column: 'completed', value: 'no' }]);
});
