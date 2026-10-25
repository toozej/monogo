'use strict';

// Run against generated preview and full-inventory directories. Playwright is a test-only dependency.
const assert = require('node:assert/strict');
const fs = require('node:fs/promises');
const path = require('node:path');
const http = require('node:http');
const { chromium } = require('playwright');

async function run(previewDirectory, fullDirectory, artifactDirectory) {
    await fs.mkdir(artifactDirectory, { recursive: true });
    const errors = [];
    const results = [];
    const server = http.createServer(async (request, response) => {
        const pathname = new URL(request.url, 'http://localhost').pathname;
        if (pathname === '/favicon.ico') { response.writeHead(204); response.end(); return; }
        const match = pathname.match(/^\/(preview|full)\/(trails\.html|app\.js|styles\.css)$/);
        if (!match) { response.writeHead(404); response.end(); return; }
        try {
            const bytes = await fs.readFile(path.join(match[1] === 'preview' ? previewDirectory : fullDirectory, match[2]));
            response.setHeader('Content-Type', match[2].endsWith('.html') ? 'text/html; charset=utf-8' : match[2].endsWith('.js') ? 'text/javascript; charset=utf-8' : 'text/css; charset=utf-8');
            response.end(bytes);
        } catch (error) { response.writeHead(500); response.end(error.message); }
    });
    await new Promise(resolve => server.listen(0, '127.0.0.1', resolve));
    const base = `http://127.0.0.1:${server.address().port}`;
    const browser = await chromium.launch({ headless: true });
    const add = text => { results.push(text); console.log(text); };
    const checkOverflow = async (page, name) => {
        const widths = await page.evaluate(() => ({ width: innerWidth, content: document.documentElement.scrollWidth }));
        assert.ok(widths.content <= widths.width, `${name} overflows horizontally: ${JSON.stringify(widths)}`);
    };
    const loaded = async page => {
        await page.waitForFunction(() => document.getElementById('results-count').textContent.startsWith('Showing'));
    };
    const observe = page => {
        page.on('pageerror', error => errors.push(error.message));
        page.on('console', message => { if (message.type() === 'error') errors.push(message.text()); });
    };
    try {
        const desktop = await browser.newContext({ viewport: { width: 1440, height: 1100 }, acceptDownloads: true, reducedMotion: 'reduce' });
        const page = await desktop.newPage();
        observe(page);
        // The test opens its own loopback server or the supplied local HTML fixture.
        // noopengrep: javascript.playwright.security.audit.playwright-goto-injection.playwright-goto-injection
        await page.goto(`${base}/preview/trails.html`);
        await loaded(page);
        const payload = await page.locator('#trail-data').evaluate(element => JSON.parse(element.textContent));
        assert.ok(payload.records.length > 1, 'The preview needs multiple inventory records.');
        assert.equal(await page.locator('#tableBody tr').count(), Math.min(24, payload.records.length));
        await checkOverflow(page, 'desktop');
        await page.screenshot({ path: path.join(artifactDirectory, 'desktop.png'), fullPage: true });
        await page.keyboard.press('/');
        assert.equal(await page.locator('#fuzzySearch').evaluate(element => element === document.activeElement), true);
        await page.locator('#fuzzySearch').fill('completed:yes');
        await page.waitForTimeout(200);
        const baseCompleted = payload.records.filter(record => record.completed).length;
        assert.equal(await page.locator('#tableBody tr').count(), Math.min(24, baseCompleted));
        await page.keyboard.press('Escape');
        await page.waitForTimeout(150);
        await page.locator('#filters-toggle').click();
        const park = payload.records.find(record => record.park)?.park;
        if (park) {
            await page.locator('#filter-park').selectOption(park);
            const count = payload.records.filter(record => record.park === park).length;
            assert.equal(await page.locator('#tableBody tr').count(), Math.min(24, count));
        }
        await page.locator('#length-min').fill('99999');
        assert.equal(await page.locator('#empty-state').isVisible(), true);
        assert.equal(await page.locator('#export-button').isDisabled(), true);
        await page.locator('#empty-reset').click();
        await page.locator('#filters-toggle').click();
        const index = payload.records.findIndex(record => !record.completed);
        assert.ok(index >= 0, 'The preview needs an uncompleted record.');
        const target = payload.records[index];
        await page.locator('#fuzzySearch').fill(`record id:"${target.id}"`);
        await page.waitForTimeout(200);
        assert.equal(await page.locator('#tableBody tr').count(), 1);
        await page.locator(`#tableBody input[data-record-key="${index}"]`).check();
        assert.equal(await page.locator('#stat-completed').textContent(), String(baseCompleted + 1));
        await page.reload();
        await loaded(page);
        assert.equal(await page.locator(`#tableBody input[data-record-key="${index}"]`).isChecked(), true);
        await page.locator(`#tableBody button[data-record-key="${index}"]`).click();
        assert.equal(await page.locator('#trail-dialog').isVisible(), true);
        assert.equal(await page.locator('#detail-id').textContent(), target.id);
        await page.locator('#completion-date').fill('2026-08-10');
        await page.locator('#completion-date').press('Tab');
        await page.keyboard.press('Escape');
        assert.equal(await page.locator('#trail-dialog').isVisible(), false);
        assert.ok((await page.locator('#tableBody').textContent()).includes('08/10/2026'));
        const downloadPromise = page.waitForEvent('download');
        await page.locator('#export-button').click();
        const download = await downloadPromise;
        const csvPath = path.join(artifactDirectory, 'filtered-progress.csv');
        await download.saveAs(csvPath);
        const csv = await fs.readFile(csvPath, 'utf8');
        assert.ok(csv.includes(target.id) && csv.includes('08/10/2026') && csv.includes('"Yes"'));
        await page.locator('#view-cards').click();
        assert.equal(await page.locator('#cards-view article').count(), 1);
        assert.equal(await page.locator('#list-view').isVisible(), false);
        await page.locator('#theme-button').click();
        await page.locator('#theme-button').click();
        assert.equal(await page.locator('html').getAttribute('data-theme'), 'dark');
        await page.locator('#fuzzySearch').fill('');
        await page.waitForTimeout(200);
        await page.evaluate(() => scrollTo(0, 0));
        await page.screenshot({ path: path.join(artifactDirectory, 'dark.png'), fullPage: true });
        add('Desktop: search, combined filters, empty state, keyboard controls, modal, saved progress, dates, CSV, cards, and dark theme passed.');

        const mobile = await browser.newContext({ viewport: { width: 390, height: 844 }, isMobile: true, hasTouch: true, deviceScaleFactor: 1, reducedMotion: 'reduce' });
        const phone = await mobile.newPage();
        observe(phone);
        // The test opens its own loopback server or the supplied local HTML fixture.
        // noopengrep: javascript.playwright.security.audit.playwright-goto-injection.playwright-goto-injection
        await phone.goto(`${base}/preview/trails.html`);
        await loaded(phone);
        assert.equal(await phone.locator('#cards-view').isVisible(), true);
        assert.equal(await phone.locator('#list-view').isVisible(), false);
        assert.ok(Number(await phone.locator('#fuzzySearch').evaluate(element => getComputedStyle(element).fontSize.replace('px', ''))) >= 16);
        assert.ok(await phone.locator('#export-button').getAttribute('aria-label'));
        await checkOverflow(phone, '390px mobile');
        await phone.screenshot({ path: path.join(artifactDirectory, 'mobile.png'), fullPage: true });
        await phone.locator('#filters-toggle').tap();
        await checkOverflow(phone, '390px mobile filters');
        const matchOption = await phone.locator('#filter-match option').nth(1).getAttribute('value');
        await phone.locator('#filter-match').selectOption(matchOption);
        await phone.locator('#reset-filters').tap();
        await phone.locator('#filters-toggle').tap();
        await phone.locator('#cards-view [data-action="details"]').first().tap();
        assert.equal(await phone.locator('#trail-dialog').isVisible(), true);
        await checkOverflow(phone, 'mobile details');
        await phone.locator('#close-dialog').tap();
        await phone.locator('#view-list').tap();
        await checkOverflow(phone, 'mobile list');
        await phone.setViewportSize({ width: 320, height: 740 });
        await phone.locator('#view-cards').tap();
        await checkOverflow(phone, '320px mobile');
        await phone.screenshot({ path: path.join(artifactDirectory, 'mobile-320.png'), fullPage: true });
        add('Mobile: automatic card view, touch filters, details, list scrolling, 16px search text, and 320px/390px layouts passed.');

        const largeContext = await browser.newContext({ viewport: { width: 1440, height: 1000 }, acceptDownloads: true });
        const large = await largeContext.newPage();
        observe(large);
        const started = Date.now();
        // The test opens its own loopback server or the supplied local HTML fixture.
        // noopengrep: javascript.playwright.security.audit.playwright-goto-injection.playwright-goto-injection
        await large.goto(`${base}/full/trails.html`);
        await loaded(large);
        const total = await large.locator('#trail-data').evaluate(element => JSON.parse(element.textContent).records.length);
        assert.ok(total > 24, 'The full inventory must exercise pagination.');
        assert.equal(await large.locator('#tableBody tr').count(), 24);
        await large.locator('#next-page').click();
        assert.equal(await large.locator('#page-number').inputValue(), '2');
        await large.locator('#page-size').selectOption('96');
        assert.equal(await large.locator('#tableBody tr').count(), Math.min(96, total));
        await large.locator('#fuzzySearch').fill('park name: Forest Park completed:no');
        await large.waitForTimeout(250);
        assert.ok(await large.locator('#tableBody tr').count() <= 96);
        await large.locator('#fuzzySearch').fill('');
        await large.waitForTimeout(200);
        const allDownloadPromise = large.waitForEvent('download');
        await large.locator('#export-button').click();
        const allDownload = await allDownloadPromise;
        const allCSVPath = path.join(artifactDirectory, 'full-inventory.csv');
        await allDownload.saveAs(allCSVPath);
        const allCSV = await fs.readFile(allCSVPath, 'utf8');
        assert.equal(allCSV.trim().split('\r\n').length, total + 1);
        add(`Full inventory: ${total} records loaded, DOM bounded to 24/96 rows, pagination and export of all records passed (${Date.now() - started} ms).`);

        const mobileLarge = await mobile.newPage();
        observe(mobileLarge);
        const session = await mobile.newCDPSession(mobileLarge);
        await session.send('Emulation.setCPUThrottlingRate', { rate: 4 });
        // The test opens its own loopback server or the supplied local HTML fixture.
        // noopengrep: javascript.playwright.security.audit.playwright-goto-injection.playwright-goto-injection
        await mobileLarge.goto(`${base}/full/trails.html`);
        await loaded(mobileLarge);
        assert.equal(await mobileLarge.locator('#cards-view article').count(), 24);
        await checkOverflow(mobileLarge, 'full inventory on mobile');
        await mobileLarge.locator('#fuzzySearch').fill('park name: Forest Park completed:no');
        await mobileLarge.waitForTimeout(400);
        assert.ok(await mobileLarge.locator('#cards-view article').count() <= 24);
        add('Full mobile inventory: card rendering and search passed with 4× CPU throttling.');

        const restricted = await browser.newContext({ viewport: { width: 1280, height: 900 } });
        await restricted.addInitScript(() => { Storage.prototype.setItem = () => { throw new DOMException('Storage unavailable', 'QuotaExceededError'); }; });
        const restrictedPage = await restricted.newPage();
        observe(restrictedPage);
        // The test opens its own loopback server or the supplied local HTML fixture.
        // noopengrep: javascript.playwright.security.audit.playwright-goto-injection.playwright-goto-injection
        await restrictedPage.goto(`${base}/preview/trails.html`);
        await loaded(restrictedPage);
        await restrictedPage.locator(`#tableBody input[data-record-key="${index}"]`).check();
        assert.ok((await restrictedPage.locator('#toast').textContent()).includes('this session'));
        assert.equal(await restrictedPage.locator('#stat-completed').textContent(), String(baseCompleted + 1));
        add('Unavailable browser storage: completion still works and the page explains that changes last for the current session.');

        const fileContext = await browser.newContext({ viewport: { width: 1280, height: 900 } });
        const filePage = await fileContext.newPage();
        observe(filePage);
        // The test opens its own loopback server or the supplied local HTML fixture.
        // noopengrep: javascript.playwright.security.audit.playwright-goto-injection.playwright-goto-injection
        await filePage.goto('file://' + path.resolve(previewDirectory, 'trails.html'));
        await loaded(filePage);
        assert.ok(await filePage.locator('#tableBody tr').count() > 0);
        assert.equal(await filePage.locator('body').evaluate(element => getComputedStyle(element).backgroundColor), 'rgb(246, 247, 242)');
        add('Standalone file: relative scripts and styles load without a web server.');
        assert.deepEqual(errors, [], 'The browser reported errors.');
        await fs.writeFile(path.join(artifactDirectory, 'browser-results.json'), JSON.stringify({ results, errors, records: total }, null, 2) + '\n');
        return { results, errors, records: total };
    } finally {
        await browser.close();
        await new Promise(resolve => server.close(resolve));
    }
}

module.exports = { run };
if (require.main === module) {
    if (process.argv.length < 4) throw new Error('Usage: node test-web-ui.cjs PREVIEW_DIRECTORY FULL_DIRECTORY [ARTIFACT_DIRECTORY]');
    run(process.argv[2], process.argv[3], process.argv[4] || '/tmp/trails-ui-results').catch(error => { console.error(error); process.exitCode = 1; });
}
