// Browser smoke test of the exported web client (the same build Telegram
// loads). Opens the game in Chromium with ?autotest=1, waits for the
// in-game autotest to finish and saves screenshots.
//
//   node scripts/web-smoke.mjs http://localhost:8088 ./shots
import { chromium } from 'playwright';

const base = process.argv[2] || 'http://localhost:8088';
const out = process.argv[3] || '.';
const user = 'web' + Math.floor(Math.random() * 1e6);

const browser = await chromium.launch({ args: ['--use-gl=angle', '--use-angle=swiftshader', '--enable-unsafe-swiftshader'] });
const page = await browser.newPage({ viewport: { width: 390, height: 844 }, deviceScaleFactor: 2, isMobile: true, hasTouch: true });
let passed = false;
page.on('console', (m) => {
  const t = m.text();
  if (t.includes('AUTOTEST') || m.type() === 'error') console.log(`[browser] ${t}`);
  if (t.includes('AUTOTEST PASS')) passed = true;
});
page.on('pageerror', (e) => console.log(`[pageerror] ${e.message}`));
await page.goto(`${base}/?dev_user=${user}&autotest=1`);
for (let i = 0; i < 90 && !passed; i++) {
  await page.waitForTimeout(2000);
  if (i === 20) await page.screenshot({ path: `${out}/web_world.png` });
}
await page.screenshot({ path: `${out}/web_final.png` });
await browser.close();
console.log(passed ? 'WEB SMOKE PASS' : 'WEB SMOKE FAIL');
process.exit(passed ? 0 : 1);
