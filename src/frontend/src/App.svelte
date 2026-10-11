<script>
  import { onMount } from 'svelte'
  import Icon from './lib/Icon.svelte'
  import Logo from './lib/Logo.svelte'
  import { app, refresh, startPolling, restoreAll, dismissNotice } from './lib/state.svelte.js'
  import Home from './pages/Home.svelte'
  import Resolver from './pages/Resolver.svelte'
  import Tests from './pages/Tests.svelte'
  import Vpn from './pages/Vpn.svelte'
  import Ghost from './pages/Ghost.svelte'

  const nav = [
    { id: 'home', label: 'Accueil', cmd: 'accueil', icon: 'home' },
    { id: 'dns', label: 'Résolveur DNS', cmd: 'dns', icon: 'dns' },
    { id: 'vpn', label: 'VPN', cmd: 'vpn', icon: 'vpn' },
    { id: 'ghost', label: 'Mode Fantôme', cmd: 'fantôme', icon: 'ghost' },
    { id: 'tests', label: 'Tests', cmd: 'tests', icon: 'tests' },
  ]

  const running = $derived(app.status?.dnsRunning ?? false)
  const systemDns = $derived(app.status?.systemDns ?? false)
  const tunnelUp = $derived(app.status?.tunnel?.up ?? false)
  const canRestore = $derived(running || systemDns || tunnelUp)

  onMount(() => {
    refresh()
    startPolling()
  })
</script>


<div class="shell">
  <nav class="sidebar" aria-label="Sections">
    <div class="brand">
      <Logo size={38} />
      <span class="brand-name label"><span class="brand-top">ARIA</span><span class="brand-sub">SHIELD</span></span>
    </div>

    <ul class="nav">
      {#each nav as item (item.id)}
        {@const lit = (item.id === 'dns' && running) || (item.id === 'vpn' && tunnelUp)}
        <li>
          <button
            class="nav-item"
            class:active={app.page === item.id}
            aria-current={app.page === item.id ? 'page' : undefined}
            aria-label={item.label}
            title={item.label}
            onclick={() => (app.page = item.id)}
          >
            <span class="nav-icon">
              <Icon name={item.icon} />
              {#if lit}<span class="dot icon-dot" aria-hidden="true"></span>{/if}
            </span>
            <span class="prompt label" aria-hidden="true">&gt;</span>
            <span class="label">{item.cmd}</span>
            {#if lit}
              <span
                class="dot label"
                class:system={item.id === 'dns' ? systemDns : app.status?.protected}
                aria-hidden="true"
              ></span>
            {/if}
          </button>
        </li>
      {/each}
    </ul>

    <div class="sidebar-foot">
      <button
        class="restore"
        disabled={!canRestore || app.busy}
        aria-label="Tout restaurer"
        title={canRestore ? 'Tout restaurer : tunnel, DNS d\'origine et arrêt du résolveur' : 'Rien à restaurer'}
        onclick={restoreAll}
      >
        <span class="nav-icon"><Icon name="restore" size={18} /></span>
        <span class="label">restaurer --tout</span>
      </button>
      <span class="version label">aria-shield v0.8</span>
    </div>
  </nav>

  <main class="content">
    {#if app.status && !app.status.admin}
      <p class="banner" role="alert">Lancée sans droits administrateur : le résolveur ne pourra pas démarrer. Relance Aria en admin.</p>
    {/if}
    {#if app.loadError}
      <p class="banner" role="alert">Impossible de lire l'état du système : {app.loadError}</p>
    {/if}
    {#if app.status?.event}
      <div class="banner info" role="status">
        <span>{app.status.event}</span>
        <button class="dismiss" aria-label="Masquer ce message" onclick={dismissNotice}>×</button>
      </div>
    {/if}

    {#if app.page === 'home'}
      <Home />
    {:else if app.page === 'dns'}
      <Resolver />
    {:else if app.page === 'vpn'}
      <Vpn />
    {:else if app.page === 'ghost'}
      <Ghost />
    {:else if app.page === 'tests'}
      <Tests />
    {/if}
  </main>
</div>

<style>
  .shell {
    display: grid;
    grid-template-columns: 216px minmax(0, 1fr);
    height: 100vh;
  }

  .sidebar {
    background: var(--sidebar);
    border-right: 1px solid var(--line-soft);
    padding: 18px 12px;
    display: flex;
    flex-direction: column;
    gap: 20px;
    min-width: 0;
  }
  .brand {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 0 4px;
    min-height: 40px;
  }
  .brand-name {
    display: flex;
    flex-direction: column;
    line-height: 1.15;
  }
  .brand-top {
    font-family: var(--font-display);
    font-weight: 700;
    font-size: 17px;
    color: #ffffff;
    letter-spacing: 0.5px;
  }
  .brand-sub {
    font-size: 11px;
    letter-spacing: 3px;
    color: var(--accent);
  }

  .nav {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 4px;
  }
  .nav-item {
    width: 100%;
    min-height: 42px;
    padding: 0 10px;
    display: flex;
    align-items: center;
    gap: 8px;
    border: none;
    border-radius: 8px 0;
    corner-shape: bevel;
    background: transparent;
    color: var(--muted);
    font-family: var(--font-mono);
    font-size: 14px;
    font-weight: 500;
    text-align: left;
  }
  .nav-item:hover:not(:disabled) {
    background: var(--hover);
    color: var(--text);
  }
  .nav-item.active {
    background: #0b1a30;
    color: var(--accent);
  }
  .prompt {
    color: var(--dim);
  }
  .nav-item.active .prompt {
    color: var(--action);
  }
  /* En grand, la console suffit : les icônes servent quand la barre est repliée */
  .nav-icon {
    position: relative;
    display: none;
    flex: none;
  }
  .dot {
    flex: none;
    margin-left: auto;
    width: 7px;
    height: 7px;
    border-radius: 50%;
    background: var(--accent);
    opacity: 0.65;
  }
  .dot.system {
    opacity: 1;
    box-shadow: 0 0 8px var(--accent);
  }
  .icon-dot {
    position: absolute;
    right: -3px;
    top: -3px;
    margin: 0;
    opacity: 1;
    border: 2px solid var(--sidebar);
    width: 9px;
    height: 9px;
  }

  .banner.info {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 12px;
    background: color-mix(in srgb, var(--accent) 8%, transparent);
    border-color: color-mix(in srgb, var(--accent) 40%, transparent);
    color: #bdf9ff;
  }
  .dismiss {
    flex: none;
    width: 28px;
    height: 28px;
    border: none;
    border-radius: 8px;
    background: transparent;
    color: inherit;
    font-size: 18px;
    line-height: 1;
  }
  .dismiss:hover {
    background: rgba(255, 255, 255, 0.08);
  }
  .label {
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .sidebar-foot {
    margin-top: auto;
    display: flex;
    flex-direction: column;
    gap: 8px;
  }
  .restore {
    min-height: 40px;
    padding: 0 10px;
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 8px;
    border: 1px solid var(--line-strong);
    border-radius: 8px 0;
    corner-shape: bevel;
    background: transparent;
    color: var(--text);
    font-family: var(--font-mono);
    font-size: 13px;
  }
  .restore:hover:not(:disabled) {
    border-color: var(--action);
    color: var(--action);
  }
  .restore:disabled {
    opacity: 0.45;
  }
  .version {
    padding: 0 4px;
    font-family: var(--font-mono);
    font-size: 11px;
    color: var(--faint);
  }

  .content {
    overflow-y: auto;
    padding: 26px 30px 30px;
    display: flex;
    flex-direction: column;
    gap: 16px;
  }

  /* Fenêtre étroite : la barre latérale se replie en icônes */
  @media (max-width: 760px) {
    .shell {
      grid-template-columns: 68px minmax(0, 1fr);
    }
    .sidebar {
      padding: 16px 8px;
      align-items: center;
    }
    .brand {
      padding: 0;
      justify-content: center;
    }
    .label {
      display: none;
    }
    .nav-icon {
      display: flex;
    }
    .nav-item,
    .restore {
      justify-content: center;
      padding: 0;
      width: 48px;
      min-height: 48px;
    }
    .nav {
      align-items: center;
    }
    .content {
      padding: 20px;
    }
  }
</style>
