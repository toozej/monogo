const test = require('node:test');
const assert = require('node:assert/strict');

function cell(text, checked) {
    const checkbox = checked === undefined ? null : { checked };
    return {
        textContent: text,
        querySelector: () => checkbox,
    };
}

function row(type, completed) {
    const cells = [
        cell('Trail'),
        cell('Forest Park'),
        cell(type),
        cell('2.0'),
        cell('Link'),
        cell('', completed),
        cell(completed ? '07/10/2026' : '-'),
    ];
    return {
        style: { display: '' },
        querySelectorAll: selector => selector === 'td' ? cells : [],
    };
}

test('free-text search spans columns and includes completion state', () => {
    const completedModerate = row('Moderate', true);
    const incompleteModerate = row('Moderate', false);
    let onReady;
    let onInput;
    const input = {
        value: '',
        addEventListener: (_event, callback) => { onInput = callback; },
    };
    global.document = {
        addEventListener: (_event, callback) => { onReady = callback; },
        getElementById: id => id === 'tableBody'
            ? { querySelectorAll: () => [completedModerate, incompleteModerate] }
            : input,
        querySelector: () => null,
    };

    require('./app.js');
    onReady();
    input.value = 'Moderate yes';
    onInput();

    assert.equal(completedModerate.style.display, '');
    assert.equal(incompleteModerate.style.display, 'none');
    delete global.document;
});
