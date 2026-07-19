---
id: contributing-docs
title: Contributing to the Documentation
description: How to edit, validate, and submit documentation updates.
---

# Contributing to the documentation

## Documentation location

- Docusaurus site: `website/`
- Markdown docs: `website/docs/`
- Sidebar config: `website/sidebars.ts`
- Site config: `website/docusaurus.config.ts`

## Local documentation workflow

From repository root:

```bash
cd website
npm install
npm run build
```

Optional local preview:

```bash
npm run start
```

## Docs deployment model

Documentation deploys with GitHub Actions workflow:

- `.github/workflows/deploy-docs.yml`

GitHub Pages should be configured to publish from GitHub Actions.
