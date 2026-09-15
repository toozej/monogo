#!/usr/bin/env bash
set -euo pipefail
: "${BIN:?Run make demo APP=gocicle}"
: "${APP_DIR:?Run make demo APP=gocicle}"
task_tmp=$(mktemp -d)
trap 'rm -rf "$task_tmp"' EXIT

echo 'Show the installed version and validate the example job.'
"$BIN" version
"$BIN" jobs validate "$APP_DIR/examples/.gocicle.yaml"

echo 'Create a temporary encryption key without displaying its value.'
"$BIN" keygen --output "$task_tmp/keys.json"

cat > "$task_tmp/workflow.yaml" <<'YAML'
name: maintenance
on:
  schedule:
    - cron: '0 3 * * 1'
  workflow_dispatch:
jobs:
  check:
    runs-on: ubuntu-latest
    container: alpine:3.22
    env:
      MODE: maintenance
    steps:
      - uses: actions/checkout@v4
        with:
          persist-credentials: false
      - run: echo "$MODE"
      - run: test -f README.md
YAML
echo 'Translate a GitHub Actions workflow with an explicit runner mapping.'
"$BIN" translate gha "$task_tmp/workflow.yaml" \
    --source https://github.com/example/project.git --branch main \
    --labels '{"ubuntu-latest":["linux","maintenance"]}' --output "$task_tmp/gha"
"$BIN" jobs validate "$task_tmp/gha/.gocicle.yaml"
cat "$task_tmp/gha/.gocicle.yaml"

cat > "$task_tmp/Jenkinsfile" <<'GROOVY'
pipeline {
    agent { docker { image 'alpine:3.22' } }
    triggers { cron('0 3 * * 1') }
    stages {
        stage('Inspect') { steps { sh 'test -f README.md' } }
        stage('Report') { steps { sh 'echo complete' } }
    }
}
GROOVY
echo 'Translate a Jenkins Declarative pipeline and validate its job.'
"$BIN" translate jenkins "$task_tmp/Jenkinsfile" \
    --source https://github.com/example/project.git --branch main \
    --labels '{"jenkins":["linux","maintenance"]}' --output "$task_tmp/jenkins"
"$BIN" jobs validate "$task_tmp/jenkins/.gocicle.yaml"

echo 'Check that unsupported Jenkins hashed cron returns an error.'
sed "s/0 3 \* \* 1/H 3 * * 1/" "$task_tmp/Jenkinsfile" > "$task_tmp/unsupported.Jenkinsfile"
if "$BIN" translate jenkins "$task_tmp/unsupported.Jenkinsfile" \
    --source https://github.com/example/project.git \
    --labels '{"jenkins":["linux"]}' --output "$task_tmp/unsupported" \
    > "$task_tmp/unsupported.log" 2>&1; then
    echo 'The translator unexpectedly accepted hashed cron.' >&2
    exit 1
fi
cat "$task_tmp/unsupported.log"
echo 'The offline demo passed. See README.md for control plane and runner installation.'
