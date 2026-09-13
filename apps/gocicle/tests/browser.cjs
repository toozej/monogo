const assert = require('node:assert/strict');
const fs = require('node:fs');
const { chromium } = require(process.env.GOCICLE_PLAYWRIGHT_MODULE);

async function main() {
    const fixture = JSON.parse(fs.readFileSync(0, 'utf8'));
    const browser = await chromium.launch({ headless: true });
    try {
        const context = await browser.newContext({ ignoreHTTPSErrors: true, serviceWorkers: 'block' });
        await context.addCookies([
            { name: '__Host-gocicle', value: fixture.session, url: fixture.url, secure: true, httpOnly: true, sameSite: 'Lax' },
            { name: '__Host-gocicle-csrf', value: fixture.csrf, url: fixture.url, secure: true, httpOnly: false, sameSite: 'Strict' },
        ]);
        const page = await context.newPage();
        const errors = [];
        page.on('pageerror', error => errors.push(error.message));
        page.on('console', message => {
            if (message.type() === 'error' && /panic|runtime error/.test(message.text())) errors.push(message.text());
        });
        page.setDefaultTimeout(30000);
        await page.goto(fixture.url);
        await page.getByRole('button', { name: 'Jobs', exact: true }).waitFor();
        await page.getByText('Create a project', { exact: true }).click();
        await page.getByLabel('Project name', { exact: true }).fill('browser-project');
        await page.getByLabel('Git URL', { exact: true }).fill('https://example.com/repo.git');
        await page.getByLabel('Branch', { exact: true }).fill('main');

        async function change(button, path, method = 'POST') {
            const response = page.waitForResponse(r => r.url().includes(path) && r.request().method() === method);
            await page.getByRole('button', { name: button, exact: true }).click();
            const result = await response;
            assert.equal(result.status(), 200, 'API change failed: ' + path);
        }

        await change('Create project', '/api/v1/projects');
        const project = (await (await context.request.get(fixture.url + '/api/v1/projects')).json())[0];
        async function storedJob() {
            return (await (await context.request.get(fixture.url + '/api/v1/projects/' + project.id + '/jobs')).json())[0];
        }
        await page.getByRole('button', { name: 'Import', exact: true }).click();
        await page.locator('section textarea').first().fill(`apiVersion: gocicle/v1
project:
  name: browser-project
  source:
    git:
      url: https://example.com/repo.git
      branch: main
jobs:
  browser-job:
    schedule: "0 3 * * 1"
    container:
      image: alpine:3.22
    command:
      path: /bin/true
`);
        await change('Preview import', '/api/v1/jobs/import-preview');
        await change('Import disabled jobs', '/jobs/import');
        await page.getByRole('button', { name: 'Jobs', exact: true }).click();
        await page.getByRole('cell', { name: 'Disabled', exact: true }).waitFor();
        await page.getByRole('button', { name: 'Edit', exact: true }).click();
        await page.getByLabel('Enable this validated job').check();
        await change('Save job', '/api/v1/jobs/', 'PUT');
        const first = await storedJob();
        await page.getByRole('cell', { name: 'Enabled', exact: true }).waitFor();
        await change('Save job', '/api/v1/jobs/', 'PUT');
        const second = await storedJob();
        assert.equal(second.version, first.version + 1);
        await change('Preview schedule', '/api/v1/schedules/preview');
        await page.locator('pre').filter({ hasText: 'UTC' }).waitFor();
        await change('Run now', '/runs');
        await page.getByRole('button', { name: 'Runs', exact: true }).click();
        await page.getByRole('button', { name: 'Refresh', exact: true }).click();
        await page.getByRole('button', { name: 'Inspect', exact: true }).click();

        const assignmentResponse = await context.request.post(fixture.url + '/api/v1/runner/poll', {
            headers: { Authorization: 'Bearer ' + fixture.runnerToken },
        });
        assert.equal(assignmentResponse.status(), 200);
        const assignment = await assignmentResponse.json();
        assert.ok(assignment.runID);
        async function report(sequence, kind, data) {
            const response = await context.request.post(fixture.url + '/api/v1/runner/runs/' + assignment.runID + '/events', {
                headers: { Authorization: 'Bearer ' + fixture.runnerToken, 'X-Gocicle-Lease': assignment.leaseToken },
                data: { sequence, kind, data },
            });
            assert.equal(response.status(), 200, await response.text());
        }
        await report(1, 'log', 'browser-live-log\n');
        await page.locator('.logs').filter({ hasText: 'browser-live-log' }).waitFor();
        await report(2, 'metrics', { available: true, memory: 1024, cpu: 1, at: new Date().toISOString() });
        await page.locator('pre').filter({ hasText: /"memory"\s*:\s*1024/ }).waitFor();
        await report(3, 'complete', { status: 'success', exitCode: 0, pushResult: 'disabled', duration: 1 });
        await page.getByRole('cell', { name: 'success', exact: true }).waitFor();
        assert.deepEqual(errors, []);
        assert.ok(project.id);
        await context.close();
    } finally {
        await browser.close();
    }
}

main().catch(error => { console.error(error); process.exitCode = 1; });
