'use strict';
// Regression test: Pages should use native GitHub Actions deployment, not an
// authentication-dependent gh api preflight that hides 403 and 404 outcomes.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const yaml = fs.readFileSync('.github/workflows/pages.yml', 'utf8');
assert.match(yaml, /actions\/upload-pages-artifact@v4/, 'missing Pages artifact upload');
assert.match(yaml, /actions\/configure-pages@v5/, 'missing supported Pages configure action');
assert.match(yaml, /actions\/deploy-pages@v4/, 'missing supported Pages deployment action');
assert.match(yaml, /needs: build/, 'Pages deployment must wait for successful build');
assert.match(yaml, /pages: write/, 'Pages deployment token needs write access');
assert.match(yaml, /id-token: write/, 'Pages deployment token needs OIDC access');
assert.doesNotMatch(yaml, /\bgh api\s+['"]?repos\/\$GITHUB_REPOSITORY\/pages/, 'custom Pages preflight masks 403 and 404 and prevents deployment');
assert.doesNotMatch(yaml, /enablement:\s*true/, 'default GITHUB_TOKEN cannot create Pages site');
console.log('PASS: GitHub Pages native artifact->configure->deploy flow without API preflight');
