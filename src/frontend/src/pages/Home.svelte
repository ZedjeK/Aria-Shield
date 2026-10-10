<script>
  import Icon from '../lib/Icon.svelte'
  import { app, protect, unprotect, tor, launchTor } from '../lib/state.svelte.js'

  const running = $derived(app.status?.dnsRunning ?? false)
  const systemDns = $derived(app.status?.systemDns ?? false)
  const protectedOn = $derived(app.status?.protected ?? false)
  const vpnReady = $derived(app.status?.vpn?.configured ?? false)
  const leak = $derived(app.status?.leak)
  const leakOK = $derived(!leak || (leak.v4Ok && leak.v6Ok))
  // Niveau affiché : 2 = protégé, 1 = DNS seulement, 0 = rien
  const level = $derived(protectedOn ? 2 : systemDns ? 1 : 0)
  const live = $derived(app.mode === 'fantome' ? 'var(--ghost)' : 'var(--accent)')
  const components = $derived(app.status?.components ?? [])

  // Où mène chaque moteur quand on clique dessus
  const target = { unbound: 'dns', wireguard: 'vpn', tor: 'ghost' }
</script>

<div class="page">
  <div class="page-head">
    <h1>Accueil</h1>
  </div>

  <div class="grid-2 home-grid">
    <section class="card hero">
      <div class="ring" class:on={level > 0} class:warn={protectedOn && !leakOK}>
        {#if level > 0}
          <svg width="56" height="56" viewBox="0 0 24 24" fill="none" stroke="var(--accent)" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M12 3l7 3v5c0 4.5-3 8.3-7 10-4-1.7-7-5.5-7-10V6z" /><path d="M9 12l2 2 4-4" /></svg>
        {:else}
          <svg width="56" height="56" viewBox="0 0 24 24" fill="none" stroke="var(--faint)" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M12 3l7 3v5c0 4.5-3 8.3-7 10-4-1.7-7-5.5-7-10V6z" /><path d="M12 8v4" /><path d="M12 15.5v0.5" /></svg>
        {/if}
      </div>
      <p class="status" class:on={level > 0}>{level === 2 ? 'Protégé' : level === 1 ? 'DNS protégé' : 'Non protégé'}</p>
      <p class="sub">
        {#if protectedOn && !leakOK}
          Attention : {leak.detail}. Vérifie la page Tests.
        {:else if protectedOn}
          Ton trafic passe par le tunnel WireGuard vers {app.status.vpn.name}. Le kill switch bloque tout le reste, et Unbound résout à travers le tunnel.
        {:else if systemDns}
          Windows passe par ton résolveur local, avec DNSSEC et sans DNS tiers. Ton IP reste visible tant que le VPN n'est pas activé.
        {:else if running}
          Ton résolveur tourne. Active « Utiliser pour tout Windows » sur la page Résolveur DNS pour que Windows s'en serve.
        {:else}
          Ton trafic sort en clair par ta box. Commence par démarrer le résolveur DNS intégré.
        {/if}
      </p>
      <div class="facts">
        <div class="fact">
          <span class="eyebrow">IP visible</span>
          <span class="fact-value">{protectedOn ? (leak?.after?.v4 || 'Celle du VPN') : 'Celle de ta box'}</span>
        </div>
        <div class="fact">
          <span class="eyebrow">Résolution DNS</span>
          <span class="fact-value">{protectedOn ? 'Unbound via le tunnel' : systemDns ? 'Unbound local' : 'DNS du FAI'}</span>
        </div>
      </div>
    </section>

    <div class="side" style="--live: {live}">
      <section class="card">
        <span class="eyebrow">Mode</span>
        <div class="modes">
          <button class="mode" class:selected={app.mode === 'discret'} style="--c: var(--accent)" onclick={() => (app.mode = 'discret')} aria-pressed={app.mode === 'discret'}>
            <span class="mode-name">Discret</span>
            <span class="mode-desc">VPN + DNS local. Rapide, pour tous les jours.</span>
          </button>
          <button class="mode" class:selected={app.mode === 'fantome'} style="--c: var(--ghost)" onclick={() => (app.mode = 'fantome')} aria-pressed={app.mode === 'fantome'}>
            <span class="mode-name">Fantôme</span>
            <span class="mode-desc">Tout passe par Tor. Plus lent, plus anonyme.</span>
          </button>
        </div>
        {#if app.mode === 'fantome'}
          <button class="cta" onclick={() => { app.page = 'ghost'; launchTor() }} disabled={tor.busy || !app.status?.admin}>
            {tor.busy ? 'En cours…' : 'Lancer Tor Browser'}
          </button>
          <p class="hint">{protectedOn ? 'Tor passera à travers ton VPN : combo idéal.' : 'Conseil : active d\'abord le mode Discret, pour que ton FAI ne voie pas Tor.'}</p>
        {:else if protectedOn}
          <button class="cta off" onclick={unprotect} disabled={app.busy}>{app.busy ? 'En cours…' : 'Désactiver la protection'}</button>
          <p class="hint">Tunnel, kill switch et DNS de Windows sont actifs. Tout revient comme avant à la désactivation.</p>
        {:else if !vpnReady}
          <button class="cta" onclick={() => (app.page = 'vpn')}>Configurer le VPN</button>
          <p class="hint">Saisis d'abord ton profil WireGuard sur la page VPN.</p>
        {:else}
          <button class="cta" onclick={protect} disabled={app.busy || !app.status?.admin}>
            {app.busy ? 'Activation… (jusqu\'à 30 s)' : 'Activer la protection'}
          </button>
          {#if app.actionError && app.logTitle === 'Activation de la protection'}
            <p class="error" role="alert">{app.actionError}</p>
          {:else}
            <p class="hint">Résolveur, DNS de Windows, tunnel vers {app.status.vpn.name} et kill switch, en un clic.</p>
          {/if}
        {/if}
      </section>

      <section class="card">
        <div class="card-head">
          <h2>Moteurs</h2>
        </div>
        {#each components as c (c.id)}
          <button class="engine" onclick={() => (app.page = target[c.id])}>
            <span class="engine-icon {c.state}">
              <Icon name={c.state === 'planned' ? 'clock' : 'check'} size={16} />
            </span>
            <span class="row-text">
              <span class="row-name">{c.name}</span>
              <span class="row-detail">{c.detail}</span>
            </span>
            <span class="chevron" aria-hidden="true">›</span>
          </button>
        {/each}
      </section>
    </div>
  </div>
</div>

<style>
  /* Les deux colonnes prennent la même hauteur */
  .home-grid {
    align-items: stretch;
  }
  .hero {
    align-items: center;
    justify-content: center;
    padding: 28px 20px 22px;
    text-align: center;
  }
  .ring {
    width: 120px;
    height: 120px;
    border-radius: 50%;
    display: flex;
    align-items: center;
    justify-content: center;
    border: 2px solid var(--line-strong);
    background: #12151a;
    transition: border-color 0.2s, box-shadow 0.2s;
  }
  .ring.on {
    border-color: var(--accent);
    background: color-mix(in srgb, var(--accent) 8%, transparent);
    box-shadow: 0 0 44px color-mix(in srgb, var(--accent) 22%, transparent);
  }
  .status.on {
    color: var(--accent);
  }
  .ring.warn {
    border-color: var(--warn);
    box-shadow: 0 0 44px rgba(245, 165, 36, 0.22);
  }
  .side[style*='ghost'] .cta {
    color: #160f2b;
  }
  .cta.off {
    background: transparent;
    color: var(--text);
    border-color: var(--line-strong);
  }
  .status {
    margin: 6px 0 0;
    font-family: var(--font-display);
    font-size: 28px;
    font-weight: 700;
  }
  .sub {
    margin: 0;
    font-size: 14px;
    line-height: 1.45;
    color: var(--muted);
    max-width: 360px;
  }
  .facts {
    width: 100%;
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 10px;
    margin-top: 8px;
    text-align: left;
  }
  .fact {
    background: var(--surface-sunken);
    border: 1px solid var(--line);
    border-radius: 12px;
    padding: 10px 12px;
    display: flex;
    flex-direction: column;
    gap: 4px;
  }
  .fact-value {
    font-family: var(--font-mono);
    font-size: 13px;
  }

  .side {
    display: flex;
    flex-direction: column;
    gap: 16px;
  }
  .modes {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 10px;
  }
  .mode {
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    gap: 6px;
    text-align: left;
    padding: 14px;
    min-height: 92px;
    border-radius: 14px;
    color: var(--text);
    background: var(--surface-sunken);
    border: 1.5px solid var(--line);
    transition: border-color 0.15s, background 0.15s;
  }
  .mode.selected {
    border-color: var(--c);
    background: color-mix(in srgb, var(--c) 10%, transparent);
  }
  .mode-name {
    font-family: var(--font-display);
    font-size: 16px;
    font-weight: 600;
  }
  .mode-desc {
    font-size: 12px;
    line-height: 1.4;
    color: var(--muted);
  }
  .cta {
    width: 100%;
    min-height: 52px;
    border-radius: 14px;
    font-size: 16px;
    font-weight: 600;
    background: var(--live);
    border: 1.5px solid var(--live);
    color: #08130e;
  }
  .cta:disabled {
    opacity: 0.45;
  }

  .engine {
    display: flex;
    align-items: center;
    gap: 12px;
    min-height: 52px;
    padding: 6px 8px;
    margin: 0 -8px;
    border: none;
    border-radius: 10px;
    background: transparent;
    color: var(--text);
    text-align: left;
  }
  .engine:hover {
    background: var(--hover);
  }
  .engine-icon {
    width: 28px;
    height: 28px;
    flex: none;
    border-radius: 8px;
    display: flex;
    align-items: center;
    justify-content: center;
    color: var(--accent);
    background: color-mix(in srgb, var(--accent) 14%, transparent);
  }
  .engine-icon.planned {
    color: var(--faint);
    background: var(--line-soft);
  }
  .chevron {
    color: var(--faint);
    font-size: 20px;
  }
</style>
