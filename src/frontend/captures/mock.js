// Faux ShieldService pour les captures d'écran de l'interface (voir
// vite.captures.config.js) : il remplace les liaisons générées par Wails et
// renvoie des données fictives, sans aucun backend. Il ne sert qu'à vérifier
// la mise en page à plusieurs largeurs, pendant la compilation sur GitHub ; il
// n'est jamais embarqué dans l'exe.
//
// L'état affiché se choisit dans l'adresse : ?etat=protege (défaut), dns, off.
// Adresses de documentation uniquement (203.0.113.x, 2001:db8::/32).

const etat = new URLSearchParams(location.search).get('etat') ?? 'protege'
const protege = etat === 'protege'
const dns = protege || etat === 'dns'

const profils = [
  { id: 'p1', active: true, configured: true, name: 'Proton Pays-Bas', addresses: ['10.2.0.2/32'], publicKey: 'AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=', endpoint: '203.0.113.10:51820', keepalive: 25, hasIpv6: false, hasPsk: false },
  { id: 'p2', active: false, configured: true, name: 'Mullvad Suède (Stockholm, serveur 14)', addresses: ['10.64.0.7/32', 'fd00::7/128'], publicKey: 'BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB=', endpoint: '203.0.113.44:51820', keepalive: 25, hasIpv6: true, hasPsk: true },
  { id: 'p3', active: false, configured: true, name: 'IVPN Suisse', addresses: ['10.9.0.3/32'], publicKey: 'CCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCC=', endpoint: '203.0.113.90:2049', keepalive: 25, hasIpv6: false, hasPsk: false },
]

const fuite = protege
  ? { v4Ok: true, v6Ok: true, detail: 'aucune fuite : les sites voient l\'IP du VPN', before: { v4: '198.•.•.7', v6: '' }, after: { v4: '203.0.113.10', v6: '' } }
  : null

const statut = {
  admin: true,
  dnsRunning: dns,
  systemDns: dns,
  protected: protege,
  event: etat === 'off' ? 'Tes données (profils VPN, Tor Browser) sont maintenant dans C:\\Users\\exemple\\.Aria-Shield, comme pour les autres applis Aria.' : '',
  dataDir: 'C:\\ProgramData\\AriaShield',
  userDir: 'C:\\Users\\exemple\\.Aria-Shield',
  vpn: profils[0],
  profiles: profils,
  tunnel: { up: protege, full: protege, killSwitch: protege, handshakeAgo: 12, rxBytes: 48_300_000, txBytes: 3_950_000 },
  leak: fuite,
  components: [
    { id: 'unbound', name: 'Unbound', detail: dns ? 'Intégré · v1.26.1 · actif' : 'Intégré · v1.26.1 · arrêté', state: dns ? 'active' : 'ready' },
    { id: 'wireguard', name: 'WireGuard', detail: protege ? 'Intégré · tunnel ouvert vers Proton Pays-Bas' : 'Intégré · profil « Proton Pays-Bas » prêt', state: protege ? 'active' : 'ready' },
    { id: 'tor', name: 'Tor Browser', detail: 'Intégré · v15.0.24', state: 'ready' },
  ],
  resuming: '',
  adBlock: dns,
  adPaused: false,
  logPath: 'C:\\Users\\exemple\\.Aria-Shield\\aria.log',
  newVersion: etat === 'off' ? '0.12' : '',
  version: '0.11.0',
}

const journal = {
  steps: [
    { label: 'IP de ta box mesurée', ok: true, note: '198.•.•.7' },
    { label: 'Lancement d\'Unbound', ok: true, note: 'PID 4242' },
    { label: 'Carte « Ethernet »', ok: true, note: '127.0.0.1, ::1' },
    { label: 'Tunnel « AriaShield » ouvert', ok: true, note: 'Proton Pays-Bas' },
    { label: 'Test de fuite', ok: true, note: 'aucune fuite : les sites voient l\'IP du VPN' },
  ],
  error: '',
}
const rien = { steps: [], error: '' }
const plus_tard = (v) => () => Promise.resolve(v)

export const ShieldService = {
  Status: plus_tard(statut),
  Adapters: plus_tard([
    { alias: 'Ethernet', servers: dns ? ['127.0.0.1', '::1'] : ['192.168.1.254'], onAria: dns },
    { alias: 'Wi-Fi (carte Intel Wi-Fi 6E AX211 160MHz)', servers: dns ? ['127.0.0.1'] : ['192.168.1.254', '2001:db8::1'], onAria: dns },
  ]),
  DismissNotice: plus_tard(undefined),

  StartDNS: plus_tard({ ...journal, test: { resolveOk: true, dnssecOk: true, detail: 'Résolution OK, signature DNSSEC invalide bien rejetée' } }),
  StopDNS: plus_tard(rien),
  TestDNS: plus_tard({ resolveOk: true, dnssecOk: true, detail: 'Résolution OK, signature DNSSEC invalide bien rejetée' }),
  EnableSystemDNS: plus_tard(journal),
  DisableSystemDNS: plus_tard(rien),

  SaveVPNProfile: plus_tard({ profile: profils[0], fields: {}, error: '' }),
  ImportVPNConf: plus_tard({ imported: [], steps: [], error: '', canceled: true }),
  DeleteVPNProfile: plus_tard(rien),
  UseVPNProfile: plus_tard(journal),
  TestVPN: plus_tard(journal),

  Protect: plus_tard(journal),
  Unprotect: plus_tard(rien),
  RestoreAll: plus_tard(rien),

  TestSystem: plus_tard({
    markerOk: dns, markerDetail: dns ? 'Windows passe bien par le résolveur d\'Aria' : 'Windows n\'utilise pas encore le résolveur',
    adaptersOk: dns, adaptersDetail: dns ? '2 cartes sur 2 pointent sur le résolveur' : 'Windows n\'utilise pas encore le résolveur',
    leak: fuite, killSwitch: protege,
    browsers: [
      { name: 'Opera GX', mode: 'DNS sécurisé désactivé', policy: false, bypass: false },
      { name: 'Google Chrome', mode: 'DNS sécurisé automatique (fournisseur du système)', policy: false, bypass: true },
      { name: 'Mozilla Firefox', mode: 'DNS via HTTPS : protection par défaut', policy: false, bypass: true },
    ],
    adBlockOn: dns, adBlockOk: dns, adBlockDetail: dns ? 'La liste est chargée : 60 312 domaines bloqués' : 'Blocage des pubs désactivé',
  }),

  TorState: plus_tard({ installed: true, version: '15.0.24', running: false, dir: 'C:\\Users\\exemple\\.Aria-Shield\\TorBrowser', progress: { active: false, phase: '', done: 0, total: 0 } }),
  TorPrepare: plus_tard(rien),
  TorCheckUpdate: plus_tard({ checked: true, available: false, current: '15.0.24', latest: '15.0.24', error: '' }),
  TorUpdate: plus_tard(rien),
  TorLaunch: plus_tard(rien),

  Settings: plus_tard({ autostart: { enabled: true, exe: 'C:\\Program Files\\Aria Shield\\Aria-Shield.exe' }, resume: true, last: protege ? 'discret' : 'dns', notify: true }),
  SetAutostart: plus_tard(rien),
  SetResume: plus_tard(rien),
  SetNotifications: plus_tard(rien),
  Quit: plus_tard(undefined),

  UpdateInfo: plus_tard({ enabled: true, checked: true, current: '0.11.0', latest: etat === 'off' ? '0.12' : '0.11.0', available: etat === 'off', url: 'https://github.com/ZedjeK/Aria-Shield/releases/latest', error: '' }),
  CheckUpdate: plus_tard({ enabled: true, checked: true, current: '0.11.0', latest: '0.11.0', available: false, url: 'https://github.com/ZedjeK/Aria-Shield/releases/latest', error: '' }),
  SetUpdateCheck: plus_tard(rien),

  AdBlock: plus_tard({
    enabled: dns, active: dns,
    list: { present: true, count: 60312, updated: '2026-11-11T07:05:21Z', source: 'OISD small', sourceId: 'small' },
    allow: ['suivi.transporteur-exemple.fr', 'liens.lettre-information-tres-longue.exemple.org'],
    pausedUntil: '',
    choice: 'small',
    sources: [
      { id: 'small', name: 'OISD small', url: '', note: 'Pubs et traceurs, peu de faux positifs. Environ 60 000 domaines, 15 Mo de mémoire en plus.' },
      { id: 'big', name: 'OISD big', url: '', note: 'Bloque plus large (pubs, traceurs, hameçonnage, logiciels malveillants). Environ 225 000 domaines, 70 Mo de mémoire en plus, et plus de sites qui cassent.' },
    ],
  }),
  SetAdBlock: plus_tard(journal),
  UpdateBlocklist: plus_tard(rien),
  SetBlocklist: plus_tard(rien),
  AddAllow: plus_tard(rien),
  RemoveAllow: plus_tard(rien),
  PauseAdBlock: plus_tard(rien),
  ResumeAdBlock: plus_tard(rien),
}
