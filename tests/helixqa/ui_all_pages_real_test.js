const { chromium } = require('playwright');
const http = require('http');
const REC = '/Volumes/T7/Downloads/Recordings';

// Get real token
function getToken() {
  return new Promise((r) => {
    const b = JSON.stringify({action:'authenticate',data:{username:'admin',password:'admin1234'}});
    const req = http.request({hostname:'localhost',port:8080,path:'/do',method:'POST',headers:{'Content-Type':'application/json'}},
      res => { let d=''; res.on('data',c=>d+=c); res.on('end',() => r(JSON.parse(d)?.data?.token||'')); });
    req.write(b); req.end();
  });
}

(async () => {
  const token = await getToken();
  if (!token) { console.log('FAIL: no token'); process.exit(1); }
  console.log('TOKEN:', token.substring(0,30)+'...');

  const browser = await chromium.launch({ headless: true });

  // Use addInitScript to set localStorage BEFORE Angular boots
  const ctx = await browser.newContext({ viewport: { width: 1280, height: 900 } });
  await ctx.addInitScript((tok) => {
    // Set backend config so Angular doesn't need service discovery
    localStorage.setItem('helixtrack_backend_config', JSON.stringify({
      serverUrl: 'http://localhost:8080',
      isCustom: true,
      discoveryEnabled: false
    }));
    // Set auth tokens
    localStorage.setItem('helixtrack_auth_tokens', JSON.stringify({
      accessToken: tok,
      refreshToken: tok,
      expiresIn: 86400,
      tokenType: 'Bearer'
    }));
    // Set user data
    localStorage.setItem('helixtrack_user_data', JSON.stringify({
      username: 'admin',
      email: 'admin@helix.test',
      name: 'Admin',
      role: 'user'
    }));
  }, token);

  const page = await ctx.newPage();
  page.on('console', msg => {
    if (msg.type() === 'error' && !msg.text().includes('ERR_CONNECTION_REFUSED'))
      console.log('ERR:', msg.text().slice(0,150));
  });

  async function tryRoute(name, path) {
    await page.goto('http://localhost:4300' + path, { waitUntil: 'domcontentloaded', timeout: 15000 }).catch(() => {});
    await page.waitForTimeout(3000);
    const body = await page.textContent('body') || '';
    const clean = body.replace(/\s+/g, ' ').trim();
    const loaded = body.length > 300 && !clean.includes('Sign In');
    const ota = (body.match(/\[OTA-\d+\]/g) || []).length;
    console.log(name + ':', loaded ? 'PASS'+ (ota?' ('+ota+' OTA)':'') : 'FAIL', '('+body.length+' chars)');
    if (loaded) await page.screenshot({ path: REC + '/helix_ota---ui-' + path.replace('/','') + '---real.png' });
    return loaded;
  }

  console.log('\n=== NAVIGATING PAGES ===');
  let ok = 0, total = 0;
  for (const [name, path] of [['Dashboard','/dashboard'],['Tickets','/tickets'],['Projects','/projects'],['Teams','/teams'],['Users','/users'],['Settings','/settings']]) {
    total++;
    if (await tryRoute(name, path)) ok++;
  }
  console.log('\n=== RESULT ===', ok+'/'+total, 'pages with real content');
  await browser.close();
})();
