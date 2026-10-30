# Spec: <short-name>

Target: <URL or /api/v1/proxy/http/<host>-<port>/… or /api/v1/projects/<id>/raw/…>
Scope: web
Engine: browser

## Acceptance

- <observable success condition>
- <forbidden state that must not appear>

## Scenarios

### S1: happy path

- goto <path-or-url>
- click <intent: e.g. the "Sign in" button>
- type "<text>" into <field intent>
- verify <checkable condition: visible text, URL, or role+name>

### S2: <edge or negative>

- goto <path-or-url>
- click <intent>
- verify not <forbidden condition>
