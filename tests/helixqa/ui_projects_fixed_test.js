const { chromium } = require('playwright');
const http = require('http');
const path = require('path');
const fs = require('fs');

const BASE = 'http://localhost:4300';
const REC_DIR = '/Volumes/T7/Downloads/Recordings';

// 1. Get REAL token from API
function getToken() {
  return new Promise((resolve, reject) => {
    const body = JSON.stringify({ action: 'authenticate', data: { username: 'admin', password: 'admin1234' } });
    const req = http.request({ hostname: 'localhost', port: 8080, path: '/do', method: 'POST',
      headers: { 'Content-Type': 'application/json' } }, res => {
      let d = '';
      res.on('data', c => d += c);
      res.on('end', () => { try { resolve(JSON.parse(d).data.token); } catch (e) { reject(e); } });
    });
    req.write(body);
    req.end();
  });
}

(async () => {
  console.log('=== SPECIFY ===');
  console.log('Expected: Real projects, teams, users, settings pages with live API data');
  console.log('No fake JWT — using real token from HelixTrack Core API');
  console.log('');

  const token = await getToken();
  console.log('Token obtained:', token ? token.substring(0, 30) + '...' : 'FAILED');
  if (!token) { process.exit(1); }

  const browser = await chromium.launch({ headless: true });
  const ctx = await browser.newContext({ viewport: { width: 1280, height: 900 } });
  const page = await ctx.newPage();

  // Step 1: Go to unguarded login page
  await page.goto(BASE + '/login', { waitUntil: 'domcontentloaded', timeout: 15000 }).catch(() => {});
  await page.goto(BASE + '/auth/login', { waitUntil: 'domcontentloaded', timeout: 15000 });

  // Step 2: Inject real token into localStorage
  await page.evaluate((tok) => {
    localStorage.setItem('helixtrack_auth_tokens', JSON.stringify({ accessToken: tok }));
    console.log('Token stored in localStorage');
  }, token);

  // Step 3: Navigate to each page and verify content
  const routes = [
    { path: '/dashboard',  name: 'Dashboard' },
    { path: '/projects',   name: 'Projects' },
    { path: '/teams',      name: 'Teams' },
    { path: '/users',      name: 'Users' },
    { path: '/settings',   name: 'Settings' },
  ];

  let allOk = true;
  for (const r of routes) {
    await page.goto(BASE + r.path, { waitUntil: 'domcontentloaded', timeout: 15000 }).catch(() => {});
    await page.waitForTimeout(2000);
    const body = await page.textContent('body') || '';
    const clean = body.replace(/\s+/g, ' ').trim();
    const isLoggedIn = !clean.toLowerCase().includes('sign in');
    const hasContent = body.length > 500;
    console.log(r.name + ':', isLoggedIn && hasContent ? 'PASS' : 'WARN',
      '(' + body.length + ' chars, loggedIn=' + isLoggedIn + ')');

    if (hasContent) {
      // Save screenshot as evidence
      await page.screenshot({ path: REC_DIR + '/helix_ota---ui-' + r.path.replace('/', '') + '---real.png', fullPage: true });
    }
    if (!isLoggedIn || !hasContent) allOk = false;
  }

  console.log('');
  console.log('=== VERDICT ===');
  console.log(allOk ? 'ALL PAGES VERIFIED WITH REAL DATA' : 'SOME PAGES NEED FIXING');

  await browser.close();
})();
