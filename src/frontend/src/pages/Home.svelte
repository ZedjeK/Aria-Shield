<script>
  // Page « Accueil » façon console : le niveau de protection en grand (texte
  // glitché, logo délavé tant que rien ne protège), la liste des moteurs en
  // [ok] / [--], puis le choix du mode et le bouton principal, qui s'adapte à la
  // situation (configurer le VPN, activer, désactiver, lancer Tor).
  import Logo from '../lib/Logo.svelte'
  import { app, protect, unprotect, tor, launchTor } from '../lib/state.svelte.js'

  const running = $derived(app.status?.dnsRunning ?? false)
  const systemDns = $derived(app.status?.systemDns ?? false)
  const protectedOn = $derived(app.status?.protected ?? false)
  const vpnReady = $derived(app.status?.vpn?.configured ?? false)
  const leak = $derived(app.status?.leak)
  const leakOK = $derived(!leak || (leak.v4Ok && leak.v6Ok))
  // Niveau affiché : 2 = protégé, 1 = DNS seulement, 0 = rien
  const level = $derived(protectedOn ? 2 : systemDns ? 1 : 0)
  const components = $derived(app.status?.components ?? [])

  // Où mène chaque moteur quand on clique dessus
  const target = { unbound: 'dns', wireguard: 'vpn', tor: 'ghost' }

  // Grand titre glitché et sa couleur
  const word = $derived(
    protectedOn && !leakOK ? 'ALERTE' : level === 2 ? 'PROTÉGÉ' : level === 1 ? 'DNS PROTÉGÉ' : 'NON PROTÉGÉ',
  )
  const tone = $derived(protectedOn && !leakOK ? 'warn' : level > 0 ? 'ok' : 'off')

  // Étiquettes façon console : en marche, prêt, à configurer.
  const tags = { active: '[ok]', ready: '[--]', planned: '[  ]' }
  // Lignes ajoutées après les moteurs du Go : kill switch et test de fuite.
  const extra = $derived([
    {
      id: 'killswitch',
      page: 'vpn',
      tag: protectedOn ? 'active' : 'ready',
      name: 'kill switch',
      detail: protectedOn ? 'filtres WFP actifs' : 'inactif',
    },
    protectedOn
      ? {
          id: 'leak',
          page: 'tests',
          tag: leakOK ? 'active' : 'alert',
          name: 'fuites',
          detail: leakOK ? 'IPv4 et IPv6 étanches' : leak.detail,
        }
      : { id: 'leak', page: 'tests', tag: 'ready', name: 'fuites', detail: 'vérifiées à l\'activation' },
  ])
  const ip = $derived(protectedOn ? leak?.after?.v4 || 'celle du VPN' : 'celle de ta box')
  const dns = $derived(
    (protectedOn ? 'Unbound via le tunnel' : systemDns ? 'Unbound local' : 'DNS du FAI') +
      ((protectedOn || systemDns) && app.status?.adBlock ? ' · pubs bloquées' : '') +
      (app.status?.adPaused ? ' · blocage des pubs en pause' : ''),
  )
</script>

<div class="page">
  <h1 class="cmd"><span class="cwd">~/aria</span> $ statut</h1>

  <!-- Niveau de protection : mot glitché, explication, logo -->
  <section class="hero" aria-label="Niveau de protection">
    <div class="hero-text">
      <span class="eyebrow">Niveau de protection</span>
      <p class="word {tone}">{word}</p>
      <p class="sub">
        {#if protectedOn && !leakOK}
          Attention : {leak.detail}. Vérifie la page Tests.
        {:else if protectedOn}
          Tunnel WireGuard vers {app.status.vpn.name} · kill switch armé · DNS résolu par Unbound dans le tunnel.
        {:else if systemDns}
          Windows passe par ton résolveur local, avec DNSSEC et sans DNS tiers. Ton IP reste visible tant que le VPN n'est pas activé.
        {:else if running}
          Ton résolveur tourne. Active « Utiliser pour tout Windows » sur la page Résolveur DNS pour que Windows s'en serve.
        {:else}
          Ton trafic sort en clair par ta box. Active la protection, ou démarre au moins le résolveur DNS intégré.
        {/if}
      </p>
    </div>
    <div class="hero-logo" class:on={level > 0}>
      <Logo size={150} dim={level === 0} label="Logo Aria Shield" />
    </div>
  </section>

  <div class="bottom">
    <!-- Moteurs : chaque ligne mène à sa page -->
    <section class="card engines" aria-label="Moteurs">
      <h2>Moteurs</h2>
      <div class="lines">
        {#each components as c (c.id)}
          <button class="line" onclick={() => (app.page = target[c.id])}>
            <span class="tag {c.state}">{tags[c.state] ?? '[--]'}</span>
            <span class="name">{c.name.toLowerCase()}</span>
            <span class="detail">{c.detail}</span>
          </button>
        {/each}
        {#each extra as e (e.id)}
          <button class="line" onclick={() => (app.page = e.page)}>
            <span class="tag {e.tag}">{e.tag === 'alert' ? '[!!]' : tags[e.tag]}</span>
            <span class="name">{e.name}</span>
            <span class="detail">{e.detail}</span>
          </button>
        {/each}
      </div>
      <div class="facts">
        <span>ip visible <strong>{ip}</strong><span class="cursor" aria-hidden="true"></span></span>
        <span>dns <strong>{dns}</strong></span>
      </div>
    </section>

    <!-- Mode et action principale (orange) -->
    <section class="mode-panel" aria-label="Mode">
      <span class="eyebrow">Mode</span>
      <div class="modes">
        <button class="mode" class:selected={app.mode === 'discret'} onclick={() => (app.mode = 'discret')} aria-pressed={app.mode === 'discret'}>discret</button>
        <button class="mode ghost" class:selected={app.mode === 'fantome'} onclick={() => (app.mode = 'fantome')} aria-pressed={app.mode === 'fantome'}>fantôme</button>
      </div>
      <p class="mode-desc">
        {app.mode === 'fantome' ? 'Tout passe par Tor. Plus lent, plus anonyme.' : 'VPN + DNS local. Rapide, pour tous les jours.'}
      </p>

      <div class="cta-zone">
        {#if app.mode === 'fantome'}
          <button class="cta" onclick={() => { app.page = 'ghost'; launchTor() }} disabled={tor.busy || !app.status?.admin}>
            {tor.busy ? 'en cours…' : 'lancer tor browser'}
          </button>
          <p class="hint">{protectedOn ? 'Tor passera à travers ton VPN : combo idéal.' : 'Conseil : active d\'abord le mode Discret, pour que ton FAI ne voie pas Tor.'}</p>
        {:else if protectedOn}
          <button class="cta" onclick={unprotect} disabled={app.busy}>{app.busy ? 'en cours…' : 'désactiver la protection'}</button>
          <p class="hint">Tunnel, kill switch et DNS de Windows sont actifs. Tout revient comme avant à la désactivation.</p>
        {:else if !vpnReady}
          <button class="cta" onclick={() => (app.page = 'vpn')}>configurer le vpn</button>
          <p class="hint">Importe d'abord le fichier .conf de ton fournisseur sur la page VPN.</p>
        {:else}
          <button class="cta" onclick={protect} disabled={app.busy || !app.status?.admin}>
            {app.busy ? 'activation… (jusqu\'à 30 s)' : 'activer la protection'}
          </button>
          {#if app.actionError && app.logTitle === 'Activation de la protection'}
            <p class="error" role="alert">{app.actionError}</p>
          {:else}
            <p class="hint">Résolveur, DNS de Windows, tunnel vers {app.status.vpn.name} et kill switch, en un clic.</p>
          {/if}
        {/if}
      </div>
    </section>
  </div>
</div>

<style>
  .cmd {
    margin: 0;
    font-family: var(--font-mono);
    font-size: 13px;
    font-weight: 400;
    color: var(--faint);
  }
  .cwd {
    color: var(--action);
    margin-right: 6px;
  }

  .hero {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto;
    gap: 20px;
    align-items: center;
    padding: 22px 26px;
    background: var(--surface);
    border: 1px solid var(--line-strong);
    border-radius: 18px;
    corner-shape: bevel;
  }
  .hero-text {
    display: flex;
    flex-direction: column;
    gap: 10px;
    min-width: 0;
  }
  .word {
    margin: 0;
    font-family: var(--font-display);
    font-size: 52px;
    font-weight: 700;
    line-height: 1;
    color: #ffffff;
    overflow-wrap: anywhere;
  }
  /* Effet glitch : deux ombres décalées, cyan et bleu, comme le A du logo */
  .word.ok {
    text-shadow: -3px 0 0 var(--accent), 3px 0 0 var(--accent-2);
  }
  .word.off {
    text-shadow: -3px 0 0 var(--action), 3px 0 0 var(--accent-2);
  }
  .word.warn {
    color: var(--warn);
    text-shadow: -3px 0 0 var(--danger), 3px 0 0 var(--accent-2);
  }
  .sub {
    margin: 0;
    font-size: 13px;
    line-height: 1.6;
    color: var(--muted);
    max-width: 520px;
  }
  .hero-logo {
    border-radius: 50%;
    padding: 6px;
  }
  .hero-logo.on {
    background: radial-gradient(circle, rgba(125, 249, 255, 0.14) 0%, rgba(125, 249, 255, 0) 70%);
  }

  .bottom {
    display: grid;
    grid-template-columns: minmax(0, 1fr);
    gap: 16px;
  }
  @container (min-width: 640px) {
    .bottom {
      grid-template-columns: minmax(0, 1.35fr) minmax(0, 1fr);
    }
  }
  @container (max-width: 520px) {
    .hero {
      grid-template-columns: minmax(0, 1fr);
    }
    .hero-logo {
      display: none;
    }
    .word {
      font-size: 38px;
    }
  }

  .lines {
    display: flex;
    flex-direction: column;
  }
  .line {
    display: grid;
    grid-template-columns: 40px 104px minmax(0, 1fr);
    gap: 8px;
    align-items: baseline;
    min-height: 36px;
    padding: 8px 8px;
    margin: 0 -8px;
    border: none;
    border-radius: 6px 0;
    corner-shape: bevel;
    background: transparent;
    color: var(--text);
    font-family: var(--font-mono);
    font-size: 13px;
    text-align: left;
  }
  .line:hover {
    background: var(--hover);
  }
  .tag {
    white-space: pre;
    color: var(--faint);
  }
  .tag.active {
    color: var(--accent);
  }
  .tag.ready {
    color: var(--ghost);
  }
  .tag.alert {
    color: var(--warn);
  }
  .name {
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .detail {
    color: var(--muted);
    font-size: 12px;
    overflow-wrap: anywhere;
  }
  /* Fenêtre étroite : le détail de chaque moteur passe sous son nom, au lieu
     de s'écraser dans une colonne d'une douzaine de caractères. Placé après
     les règles de base de .line et .detail : à spécificité égale, c'est la
     dernière règle écrite qui s'applique. */
  @container (max-width: 420px) {
    .line {
      grid-template-columns: 40px minmax(0, 1fr);
      row-gap: 2px;
    }
    .name,
    .detail {
      grid-column: 2;
    }
  }
  .facts {
    margin-top: auto;
    padding-top: 10px;
    border-top: 1px solid var(--line-soft);
    display: flex;
    flex-direction: column;
    gap: 4px;
    font-size: 13px;
    color: var(--faint);
  }
  .facts strong {
    margin-left: 6px;
    font-weight: 500;
    color: var(--text);
  }
  /* Curseur clignotant de console (immobile si l'utilisateur limite les animations) */
  .cursor {
    display: inline-block;
    width: 8px;
    height: 15px;
    margin-left: 6px;
    vertical-align: -2px;
    background: var(--action);
    animation: blink 1.1s steps(1) infinite;
  }
  @keyframes blink {
    50% {
      opacity: 0;
    }
  }

  .mode-panel {
    display: flex;
    flex-direction: column;
    gap: 12px;
    min-width: 0;
  }
  .modes {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 8px;
  }
  .mode {
    --c: var(--accent);
    min-height: 60px;
    border: 1.5px solid var(--line-strong);
    border-radius: 10px 0;
    corner-shape: bevel;
    background: transparent;
    color: var(--text);
    font-family: var(--font-mono);
    font-size: 14px;
    font-weight: 600;
  }
  .mode.ghost {
    --c: var(--ghost);
  }
  .mode:hover:not(.selected) {
    background: var(--hover);
  }
  .mode.selected {
    border-color: var(--c);
    color: var(--c);
    background: color-mix(in srgb, var(--c) 10%, transparent);
  }
  .mode-desc {
    margin: 0;
    font-size: 12px;
    line-height: 1.6;
    color: var(--muted);
  }
  .cta-zone {
    margin-top: auto;
    display: flex;
    flex-direction: column;
    gap: 8px;
  }
  .cta {
    width: 100%;
    min-height: 52px;
    border: none;
    border-radius: 12px 0;
    corner-shape: bevel;
    background: var(--action);
    color: var(--on-action);
    font-family: var(--font-mono);
    font-size: 15px;
    font-weight: 600;
  }
  .cta:hover:not(:disabled) {
    background: #ff9d42;
  }
  .cta:disabled {
    opacity: 0.45;
  }
</style>
