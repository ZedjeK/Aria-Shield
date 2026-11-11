// État partagé de l'interface et appels au backend Go.
//
// Toutes les pages lisent `app` (et `tor` pour le Mode Fantôme) : ce sont des
// objets réactifs Svelte 5 ($state), donc chaque changement redessine ce qui
// en dépend. Les fonctions exportées sont les seules à appeler ShieldService
// (le Go, via les liaisons générées par Wails dans bindings/).
//
// Convention : une action longue passe par run(), qui bloque les boutons
// (app.busy), range le journal de l'action dans app.log et relit l'état.
import { ShieldService } from '../../bindings/aria-shield'

/** État général de l'interface. */
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

/** Relit l'état DNS des cartes réseau (page Résolveur DNS). */
export async function loadAdapters() {
  try {
    app.adapters = (await ShieldService.Adapters()) ?? []
  } catch {
    app.adapters = []
  }
}

// Texte lisible d'une erreur, qu'elle vienne du Go ou du JavaScript.
const message = (err) => String(err?.message ?? err)

/** Masque le bandeau d'information (incident DNS, migration…). */
export async function dismissNotice() {
  try {
    await ShieldService.DismissNotice()
  } catch {}
  refresh()
}

/** Relit l'état complet (droits, moteurs, protection, profils). */
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

// Exécute une action du backend : boutons bloqués pendant l'action, journal
// rangé sous le titre donné, message d'erreur affiché, état relu à la fin.
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

// ---------- Résolveur ----------

/** Démarre Unbound puis affiche ses tests. */
export async function startDNS() {
  app.test = null
  app.sysTests = null
  const res = await run('Démarrage du résolveur', () => ShieldService.StartDNS())
  if (res && !res.error) app.test = res.test
}

/** Arrête Unbound. */
export async function stopDNS() {
  await run('Arrêt du résolveur', () => ShieldService.StopDNS())
  app.test = null
  app.sysTests = null
}

/** Relance les tests du résolveur (résolution et DNSSEC). */
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

/** Fait utiliser le résolveur par tout Windows, puis le vérifie. */
export async function enableSystemDNS() {
  const res = await run('Bascule de Windows sur le résolveur', () => ShieldService.EnableSystemDNS())
  if (res && !res.error) await testSystem()
}

/** Rend son DNS d'origine à Windows. */
export async function disableSystemDNS() {
  await run('Retour au DNS d\'origine', () => ShieldService.DisableSystemDNS())
  app.sysTests = null
}

// ---------- VPN ----------

/**
 * Enregistre un profil VPN saisi à la main. id vide = nouveau profil.
 * Renvoie les erreurs par champ (res.fields) pour le formulaire.
 */
export async function saveProfile(id, input) {
  app.busy = true
  app.actionError = ''
  try {
    const res = await ShieldService.SaveVPNProfile(id ?? '', input)
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

/** Supprime un profil (refusé par le Go s'il sert au tunnel ouvert). */
export async function deleteProfile(id) {
  await run('Suppression du profil VPN', () => ShieldService.DeleteVPNProfile(id))
}

/** Choisit le profil actif ; protection active, bascule à chaud vers ce serveur. */
export async function useProfile(id) {
  const protecting = app.status?.protected
  await run(protecting ? 'Changement de serveur' : 'Choix du profil VPN', () => ShieldService.UseVPNProfile(id))
}

// Le sélecteur de fichiers est celui de Windows ; Go lit les fichiers lui-même.
export async function importConf() {
  const res = await run('Import de configurations WireGuard', () => ShieldService.ImportVPNConf())
  return res
}

/** Ouvre le tunnel du profil actif quelques secondes pour vérifier les clés. */
export async function testVPN() {
  await run('Test de connexion au serveur VPN', () => ShieldService.TestVPN())
}

// ---------- Protection ----------

/** Active le mode Discret : résolveur, DNS de Windows, tunnel et kill switch. */
export async function protect() {
  app.sysTests = null
  await run('Activation de la protection', () => ShieldService.Protect())
}

/** Désactive la protection et rend sa connexion d'origine à Windows. */
export async function unprotect() {
  await run('Désactivation de la protection', () => ShieldService.Unprotect())
  app.sysTests = null
}

// ---------- Mise en forme ----------

/** 1536 → « 1,5 Ko » (unités françaises, virgule décimale). */
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

/** Secondes écoulées → « il y a 12 s », « il y a 3 min »… */
export function formatAgo(s) {
  if (s === undefined || s === null || s < 0) return 'jamais'
  if (s < 5) return 'à l\'instant'
  if (s < 60) return `il y a ${s} s`
  return `il y a ${Math.floor(s / 60)} min`
}

// ---------- Mode Fantôme ----------

/** État de Tor Browser et de ses mises à jour (page Mode Fantôme). */
export const tor = $state({
  state: null, // installé, version, ouvert, progression
  update: null, // résultat de la vérification
  prompt: false, // proposition de mise à jour affichée
  busy: false,
  error: '',
  log: [],
})

// Pendant une installation ou une mise à jour, l'état est relu toutes les
// 400 ms pour faire avancer la barre de progression.
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

/** Relit l'état de Tor Browser (installé, version, ouvert). */
export async function refreshTor() {
  try {
    tor.state = await ShieldService.TorState()
  } catch (err) {
    tor.error = message(err)
  }
}

// Une étape Tor : son journal s'ajoute à la suite, une erreur arrête la séquence.
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

/** Met à jour Tor Browser (version signée vérifiée), puis le lance. */
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

/** Lance la version installée sans mettre à jour (choix de l'utilisateur). */
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

/** Vérifie seulement s'il existe une nouvelle version. */
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

/** « restaurer --tout » : tunnel fermé, DNS d'origine, résolveur arrêté. */
export async function restoreAll() {
  await run('Restauration complète', () => ShieldService.RestoreAll())
  app.test = null
  app.sysTests = null
  app.page = 'dns'
}

// ---------- Nouvelles versions ----------

/** Dernière vérification des versions (page Réglages). */
export const update = $state({
  data: null, // { enabled, checked, current, latest, available, url, error }
  busy: false,
})

/** Relit le résultat de la dernière vérification (sans réseau). */
export async function loadUpdate() {
  try {
    update.data = await ShieldService.UpdateInfo()
  } catch {}
}

/** Interroge GitHub maintenant. */
export async function checkUpdate() {
  update.busy = true
  try {
    update.data = await ShieldService.CheckUpdate()
  } catch (err) {
    update.data = { ...(update.data ?? {}), error: message(err) }
  } finally {
    update.busy = false
    refresh()
  }
}

/** Active ou coupe la vérification au lancement. */
export async function setUpdateCheck(on) {
  try {
    await ShieldService.SetUpdateCheck(on)
  } catch {}
  await loadUpdate()
}

// ---------- Blocage des pubs ----------

/** État du blocage des pubs (page Résolveur DNS). */
export const ads = $state({
  data: null, // { enabled, active, list: { present, count, updated, source }, allow, choice, sources, pausedUntil }
  error: '',
})

/** Relit l'état du blocage. */
export async function loadAds() {
  try {
    ads.data = await ShieldService.AdBlock()
    ads.error = ''
  } catch (err) {
    ads.error = message(err)
  }
}

/** Active ou coupe le blocage (télécharge la liste au besoin, redémarre Unbound). */
export async function setAdBlock(on) {
  await run(on ? 'Activation du blocage des pubs' : 'Arrêt du blocage des pubs', () => ShieldService.SetAdBlock(on))
  await loadAds()
  app.sysTests = null
}

/** Retélécharge la liste maintenant. */
export async function updateBlocklist() {
  await run('Mise à jour de la liste de blocage', () => ShieldService.UpdateBlocklist())
  await loadAds()
}

/** Ajoute une exception. Renvoie true si elle a été acceptée. */
export async function addAllow(domain) {
  const res = await run('Exception au blocage', () => ShieldService.AddAllow(domain))
  await loadAds()
  return Boolean(res && !res.error)
}

/** Choisit la liste de blocage (« small » ou « big »). */
export async function setBlocklist(id) {
  await run('Choix de la liste de blocage', () => ShieldService.SetBlocklist(id))
  await loadAds()
}

/** Suspend le blocage quelques minutes ; il reprend tout seul. */
export async function pauseAdBlock(minutes) {
  await run('Suspension du blocage des pubs', () => ShieldService.PauseAdBlock(minutes))
  await loadAds()
}

/** Rétablit le blocage avant la fin de la suspension. */
export async function resumeAdBlock() {
  await run('Reprise du blocage des pubs', () => ShieldService.ResumeAdBlock())
  await loadAds()
}

/** Retire une exception. */
export async function removeAllow(domain) {
  await run('Exception au blocage', () => ShieldService.RemoveAllow(domain))
  await loadAds()
}

// ---------- Réglages ----------

/** Réglages de démarrage (lus à l'ouverture de la page Réglages). */
export const settings = $state({
  data: null, // { autostart: { enabled, exe }, resume, last }
  error: '',
})

/** Relit les réglages (interroge le Planificateur de tâches : pas en boucle). */
export async function loadSettings() {
  try {
    settings.data = await ShieldService.Settings()
    settings.error = ''
  } catch (err) {
    settings.error = message(err)
  }
}

/** Crée ou supprime la tâche qui lance Aria à l'ouverture de session. */
export async function setAutostart(on) {
  await run(on ? 'Démarrage avec Windows' : 'Arrêt du démarrage avec Windows', () => ShieldService.SetAutostart(on))
  await loadSettings()
}

/** Active ou non la réactivation de la dernière protection au lancement. */
export async function setResume(on) {
  await run('Réactivation au lancement', () => ShieldService.SetResume(on))
  await loadSettings()
}

/** Active ou non les notifications Windows (incidents, sortie d'Aria). */
export async function setNotifications(on) {
  await run('Notifications Windows', () => ShieldService.SetNotifications(on))
  await loadSettings()
}

/** Quitte vraiment Aria : la connexion d'origine est remise par le Go. */
export async function quitApp() {
  app.busy = true
  try {
    await ShieldService.Quit()
  } catch {}
}

// ---------- Tests ----------

/** Tests côté Windows : marqueur DNS, cartes, fuites, navigateurs. */
export async function testSystem() {
  try {
    app.sysTests = await ShieldService.TestSystem()
  } catch (err) {
    app.actionError = message(err)
  }
}

/** Tous les tests d'un coup (page Tests). */
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

/** Libellé et style d'une pastille de résultat de test. */
export function chip(ok, pending) {
  if (pending) return { label: 'En attente', cls: 'idle' }
  return ok ? { label: 'OK', cls: 'ok' } : { label: 'Échec', cls: 'fail' }
}
