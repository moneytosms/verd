# verd

## Release notes

Release bodies are read by `verd update`, so they are for users, not git history.

- Write each release's notes by hand. Do not ship goreleaser's auto-generated commit list (`changelog: use: github`).
- Plain language: what a user can now do, what got fixed, what changed for them. Skip internal refactors unless they affect users.
- Group under short headings (`New`, `Fixed`, `Changed`). Omit empty ones.
- One line per item. No commit hashes, no `feat:`/`fix:` prefixes, no author handles.
- Set the body with `gh release edit vX.Y.Z --notes-file <file>` after the release job creates it.
