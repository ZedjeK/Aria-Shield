// État partagé de l'interface et appels au backend Go.
import { ShieldService } from '../../bindings/aria-shield'

export const app = $state({
  page: 'home',
  mode: 'discret',
  status: null,
  loadError: '',
  busy: false,
  // Journal de la dernière action (démarrage, bascule, restauration)
  logTitle: '',
  log: [],
  actionError: '',
  test: null, // tests du résolveur
  sysTests: null, // tests côté Windows
  adapters: [], // cartes réseau actives
})

export async function loadAdapters() {
  try {
    app.adapters = (await ShieldService.Adapters()) ?? []
  } catch {
    app.adapters = []
  }
}

const message = (err) => String(err?.message ?? err)

export async function dismissNotice() {
  try {
    await ShieldService.DismissNotice()
  } catch {}
  refresh()
}

export async function refresh() {
  try {
    app.status = await ShieldService.Status()
    app.loadError = ''
  } catch (err) {
    app.loadError = message(err)
  }
}

// Rafraîchit l'état régulièrement : une restauration automatique doit se voir.
let timer = null
export function startPolling() {
  if (timer) return
  timer = setInterval(() => {
    if (!app.busy) refresh()
  }, 5000)
}

async function run(title, fn) {
  app.busy = true
  app.actionError = ''
  try {
    const res = await fn()
    if (res?.steps) {
      app.logTitle = title
      app.log = res.steps
    }
    if (res?.error) app.actionError = res.error
    return res
  } catch (err) {
    app.actionError = message(err)
  } finally {
    app.busy = false
    await refresh()
    loadAdapters()
  }
}

export async function startDNS() {
  app.test = null
  app.sysTests = null
  const res = await run('Démarrage du résolveur', () => ShieldService.StartDNS())
  if (res && !res.error) app.test = res.test
}

export async function stopDNS() {
  await run('Arrêt du résolveur', () => ShieldService.StopDNS())
  app.test = null
  app.sysTests = null
}

export async function testDNS() {
  app.busy = true
  app.actionError = ''
  try {
    app.test = await ShieldService.TestDNS()
  } catch (err) {
    app.actionError = message(err)
  } finally {
    app.busy = false
  }
}

export async function enableSystemDNS() {
  const res = await run('Bascule de Windows sur le résolveur', () => ShieldService.EnableSystemDNS())
  if (res && !res.error) await testSystem()
}

export async function disableSystemDNS() {
  await run('Retour au DNS d\'origine', () => ShieldService.DisableSystemDNS())
  app.sysTests = null
}

// ---------- VPN ----------

export async function saveProfile(input) {
  app.busy = true
  app.actionError = ''
  try {
    const res = await ShieldService.SaveVPNProfile(input)
    if (res.error) app.actionError = res.error
    return res
  } catch (err) {
    app.actionError = message(err)
    return { error: app.actionError }
  } finally {
    app.busy = false
    await refresh()
  }
}

export async function deleteProfile() {
  await run('Suppression du profil VPN', () => ShieldService.DeleteVPNProfile())
}

export async function testVPN() {
  await run('Test de connexion au serveur VPN', () => ShieldService.TestVPN())
}

export async function protect() {
  app.sysTests = null
  await run('Activation de la protection', () => ShieldService.Protect())
}

export async function unprotect() {
  await run('Désactivation de la protection', () => ShieldService.Unprotect())
  app.sysTests = null
}

export function formatBytes(n) {
  if (!n) return '0 o'
  const units = ['o', 'Ko', 'Mo', 'Go', 'To']
  let i = 0
  let v = n
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024
    i++
  }
  return `${v.toFixed(i === 0 ? 0 : 1).replace('.', ',')} ${units[i]}`
}

export function formatAgo(s) {
  if (s === undefined || s === null || s < 0) return 'jamais'
  if (s < 5) return 'à l\'instant'
  if (s < 60) return `il y a ${s} s`
  return `il y a ${Math.floor(s / 60)} min`
}

// ---------- Mode Fantôme ----------

export const tor = $state({
  state: null, // installé, version, ouvert, progression
  update: null, // résultat de la vérification
  prompt: false, // proposition de mise à jour affichée
  busy: false,
  error: '',
  log: [],
})

let torTimer = null
function watchTor(on) {
  clearInterval(torTimer)
  torTimer = null
  if (on) {
    torTimer = setInterval(async () => {
      try {
        tor.state = await ShieldService.TorState()
      } catch {}
    }, 400)
  }
}

export async function refreshTor() {
  try {
    tor.state = await ShieldService.TorState()
  } catch (err) {
    tor.error = message(err)
  }
}

async function torStep(fn) {
  const res = await fn()
  if (res?.steps?.length) tor.log = [...tor.log, ...res.steps]
  if (res?.error) throw new Error(res.error)
  return res
}

// Lancer : installation au premier passage, vérification des mises à jour, puis lancement.
export async function launchTor() {
  tor.busy = true
  tor.error = ''
  tor.log = []
  tor.prompt = false
  watchTor(true)
  try {
    await torStep(() => ShieldService.TorPrepare())
    tor.update = await ShieldService.TorCheckUpdate()
    if (tor.update.available || tor.update.error) {
      tor.prompt = true // l'utilisateur choisit
      return
    }
    await torStep(() => ShieldService.TorLaunch())
  } catch (err) {
    tor.error = message(err)
  } finally {
    watchTor(false)
    tor.busy = false
    refreshTor()
    refresh()
  }
}

export async function updateAndLaunchTor() {
  tor.busy = true
  tor.error = ''
  tor.prompt = false
  watchTor(true)
  try {
    await torStep(() => ShieldService.TorUpdate())
    await torStep(() => ShieldService.TorLaunch())
    tor.update = { ...tor.update, available: false, current: tor.update.latest }
  } catch (err) {
    tor.error = message(err)
  } finally {
    watchTor(false)
    tor.busy = false
    refreshTor()
    refresh()
  }
}

export async function launchTorAnyway() {
  tor.busy = true
  tor.error = ''
  tor.prompt = false
  try {
    await torStep(() => ShieldService.TorLaunch())
  } catch (err) {
    tor.error = message(err)
  } finally {
    tor.busy = false
    refreshTor()
  }
}

export async function checkTorUpdate() {
  tor.busy = true
  tor.error = ''
  try {
    tor.update = await ShieldService.TorCheckUpdate()
    tor.prompt = Boolean(tor.update.available)
  } catch (err) {
    tor.error = message(err)
  } finally {
    tor.busy = false
  }
}

export async function restoreAll() {
  await run('Restauration complète', () => ShieldService.RestoreAll())
  app.test = null
  app.sysTests = null
  app.page = 'dns'
}

export async function testSystem() {
  try {
    app.sysTests = await ShieldService.TestSystem()
  } catch (err) {
    app.actionError = message(err)
  }
}

export async function testAll() {
  app.busy = true
  app.actionError = ''
  try {
    if (app.status?.dnsRunning) app.test = await ShieldService.TestDNS()
    app.sysTests = await ShieldService.TestSystem()
  } catch (err) {
    app.actionError = message(err)
  } finally {
    app.busy = false
  }
}

// Raison affichée quand le démarrage du résolveur est impossible.
export function blockedReason() {
  if (!app.status) return 'Lecture de l\'état en cours…'
  if (!app.status.admin) return 'Aria doit être lancée en administrateur pour démarrer le résolveur.'
  return ''
}

export function chip(ok, pending) {
  if (pending) return { label: 'En attente', cls: 'idle' }
  return ok ? { label: 'OK', cls: 'ok' } : { label: 'Échec', cls: 'fail' }
}
