// Prend des captures de chaque page de l'interface à plusieurs largeurs, avec
// le faux service de mock.js. Lancé par GitHub Actions (job « captures ») après
// « vite build » et « vite preview » ; les images sont gardées en artefact.
//
// La fenêtre réelle d'Aria fait 980 × 640 par défaut et 420 × 520 au minimum.
// Les captures « hautes » montrent toute la page d'un coup (le contenu défile
// dans la fenêtre, pas le document) ; les captures « fenêtre » montrent ce que
// l'on voit vraiment sans défiler.
import { mkdir } from 'node:fs/promises'
import { chromium } from 'playwright'

const base = process.env.CAPTURES_URL ?? 'http://localhost:4173/'
const out = process.env.CAPTURES_DIR ?? 'captures-out'

const pages = [
  ['accueil', 'Accueil'],
  ['dns', 'Résolveur DNS'],
  ['vpn', 'VPN'],
  ['fantome', 'Mode Fantôme'],
  ['tests', 'Tests'],
  ['reglages', 'Réglages'],
]
const largeurs = [420, 760, 980, 1400]

await mkdir(out, { recursive: true })
const browser = await chromium.launch()
const erreurs = []

async function ouvrir(viewport, etat) {
  const page = await browser.newPage({ viewport, colorScheme: 'dark' })
  page.on('pageerror', (e) => erreurs.push(`${etat} ${viewport.width}px : ${e.message}`))
  await page.goto(`${base}?etat=${etat}`)
  await page.waitForSelector('.shell')
  await page.evaluate(() => document.fonts.ready)
  return page
}

// Débordement horizontal : un élément plus large que la zone de contenu.
async function debordement(page) {
  return page.evaluate(() => {
    const c = document.querySelector('.content')
    return c ? c.scrollWidth - c.clientWidth : 0
  })
}

const depassements = []
for (const etat of ['protege', 'off']) {
  for (const largeur of largeurs) {
    const page = await ouvrir({ width: largeur, height: 1700 }, etat)
    for (const [nom, label] of pages) {
      if (etat === 'off' && !['accueil', 'dns', 'reglages'].includes(nom)) continue
      await page.getByRole('button', { name: label, exact: true }).click()
      await page.waitForTimeout(350)
      if (nom === 'tests') {
        await page.getByRole('button', { name: 'Relancer' }).click()
        await page.waitForTimeout(350)
      }
      const d = await debordement(page)
      if (d > 1) depassements.push(`${etat} · ${nom} · ${largeur}px : dépasse de ${d}px`)
      await page.screenshot({ path: `${out}/${etat}-${nom}-${largeur}.png` })
    }
    await page.close()
  }
}

// Ce que l'on voit sans défiler : fenêtre par défaut et fenêtre minimale.
for (const [w, h] of [[980, 640], [420, 520]]) {
  const page = await ouvrir({ width: w, height: h }, 'protege')
  for (const [nom, label] of pages) {
    await page.getByRole('button', { name: label, exact: true }).click()
    await page.waitForTimeout(350)
    await page.screenshot({ path: `${out}/fenetre-${w}x${h}-${nom}.png` })
  }
  await page.close()
}

await browser.close()

console.log(`Captures écrites dans ${out}`)
if (depassements.length) {
  console.log('\nDébordements horizontaux :')
  for (const d of depassements) console.log('::warning::' + d)
}
if (erreurs.length) {
  console.log('\nErreurs JavaScript :')
  for (const e of erreurs) console.log('::error::' + e)
  process.exit(1)
}
