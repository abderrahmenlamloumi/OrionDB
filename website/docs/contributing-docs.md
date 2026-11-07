---
id: contributing-docs
title: Contributing to Documentation
description: How to edit and build the OrionDB docs.
---

# Contributing to the Documentation

The OrionDB documentation is built using [Docusaurus 3](https://docusaurus.io/), a modern static website generator.

## Editing Content

All documentation content is written in Markdown (`.md`) and located in the `docs/` folder at the root of the project.

To make a change:
1. Edit the relevant `.md` file.
2. Ensure the frontmatter (`id`, `title`) remains intact at the top of the file.

## Running the Docs Locally

To preview your changes locally before committing:

```bash
cd website
npm ci
npm start