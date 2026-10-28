'use strict';
const form = document.getElementById('album-form');
const copy = document.getElementById('copy');
const copyFields = document.getElementById('copy-fields');
const status = document.getElementById('status');
const submit = document.getElementById('submit');
let listText = '';
function updateCopyFields() {
  copyFields.hidden = !copy.checked;
  document.getElementById('source').required = copy.checked;
  document.getElementById('destination').required = copy.checked;
}
copy.addEventListener('change', updateCopyFields);
updateCopyFields();
form.addEventListener('submit', async (event) => {
  event.preventDefault();
  submit.disabled = true;
  document.getElementById('result').hidden = true;
  status.textContent = 'Reading the album. Large albums can take several minutes.';
  const value = (id) => document.getElementById(id).value;
  try {
    const response = await fetch('/submit', {
      method: 'POST',
      headers: {'Content-Type': 'application/json', 'X-CSRF-Token': document.querySelector('meta[name="csrf-token"]').content},
      body: JSON.stringify({album: value('album'), prefix: value('prefix'), suffix: value('suffix'), copy: copy.checked, source: value('source'), destination: value('destination'), no_overwrite: document.getElementById('no-overwrite').checked})
    });
    if (!(response.headers.get('Content-Type') || '').includes('application/json')) {
      throw new Error(await response.text());
    }
    const data = await response.json();
    if (data.error && !data.result.list_ready) throw new Error(data.error);
    const result = data.result;
    listText = result.text;
    document.getElementById('preview').textContent = listText || 'No photos with hearts were found.';
    document.getElementById('summary').textContent = `${result.filenames.length} filenames · ${result.copied.length} copied · ${result.missing.length} missing · ${result.ambiguous.length} ambiguous · ${result.skipped.length} skipped`;
    const details = document.getElementById('copy-results');
    details.replaceChildren();
    for (const [key, label] of [['missing', 'Missing'], ['ambiguous', 'Ambiguous'], ['skipped', 'Skipped']]) {
      if (result[key].length) {
        const paragraph = document.createElement('p');
        paragraph.textContent = `${label}: ${result[key].join(', ')}`;
        details.append(paragraph);
      }
    }
    document.getElementById('result').hidden = false;
    status.textContent = data.error || 'The filename list is ready.';
  } catch (error) {
    status.textContent = error.message;
  } finally {
    submit.disabled = false;
  }
});
document.getElementById('download').addEventListener('click', () => {
  const url = URL.createObjectURL(new Blob([listText], {type: 'text/plain;charset=utf-8'}));
  const link = document.createElement('a');
  link.href = url;
  link.download = 'liked-photos.txt';
  link.click();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
});
