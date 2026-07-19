---
id: troubleshooting
title: Troubleshooting
description: Common issues and verified fixes for local development.
---

# Troubleshooting

## Docusaurus build fails with Node version error

Symptom:

- Error indicates minimum Node version `>=20.0`

Fix:

```bash
export NVM_DIR="$HOME/.nvm"
. "$NVM_DIR/nvm.sh"
nvm install 20
nvm use 20
node -v
```
