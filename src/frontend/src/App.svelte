<script>
  import { onMount } from 'svelte'
  import Icon from './lib/Icon.svelte'
  import { app, refresh, startPolling, restoreAll } from './lib/state.svelte.js'
  import Home from './pages/Home.svelte'
  import Resolver from './pages/Resolver.svelte'
  import Tests from './pages/Tests.svelte'
  import Vpn from './pages/Vpn.svelte'
  import Ghost from './pages/Ghost.svelte'

  const nav = [
    { id: 'home', label: 'Accueil', icon: 'home' },
    { id: 'dns', label: 'Résolveur DNS', icon: 'dns' },
    { id: 'vpn', label: 'VPN', icon: 'vpn' },
    { id: 'ghost', label: 'Mode Fantôme', icon: 'ghost' },
    { id: 'tests', label: 'Tests', icon: 'tests' },
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
      <span class="brand-icon"><Icon name="shield" size={22} /></span>
      <span class="brand-name label">Aria Shield</span>
    </div>

    <ul class="nav">
      {#each nav as item (item.id)}
        <li>
          <button
            class="nav-item"
            class:active={app.page === item.id}
            aria-current={app.page === item.id ? 'page' : undefined}
            title={item.label}
            onclick={() => (app.page = item.id)}
          >
            <span class="nav-icon">
              <Icon name={item.icon} />
              {#if item.id === 'dns' && running}
                <span class="dot" class:system={systemDns} aria-label={systemDns ? 'utilisé par Windows' : 'actif'}></span>
              {/if}
              {#if item.id === 'vpn' && tunnelUp}
                <span class="dot" class:system={app.status?.protected} aria-label="tunnel ouvert"></span>
              {/if}
            </span>
            <span class="label">{item.label}</span>
            {#if item.soon}
              <span class="soon label">Bientôt</span>
            {/if}
          </button>
        </li>
      {/each}
    </ul>

    <div class="sidebar-foot">
      <button
        class="nav-item restore"
        disabled={!canRestore || app.busy}
        title={canRestore ? 'Tout restaurer : tunnel, DNS d\'origine et arrêt du résolveur' : 'Rien à restaurer'}
        onclick={restoreAll}
      >
        <span class="nav-icon"><Icon name="restore" /></span>
        <span class="label">Tout restaurer</span>
      </button>
      <span class="version label">v0.6</span>
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
      <p class="banner info" role="status">{app.status.event}</p>
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
    grid-template-columns: 240px minmax(0, 1fr);
    height: 100vh;
  }

  .sidebar {
    background: var(--sidebar);
    border-right: 1px solid var(--line);
    padding: 16px 10px;
    display: flex;
    flex-direction: column;
    gap: 16px;
    min-width: 0;
  }
  .brand {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 4px 12px 8px;
    min-height: 40px;
  }
  .brand-icon {
    color: var(--accent);
    display: flex;
  }
  .brand-name {
    font-family: var(--font-display);
    font-weight: 600;
    font-size: 18px;
    white-space: nowrap;
  }

  .nav {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }
  .nav-item {
    position: relative;
    width: 100%;
    min-height: 44px;
    padding: 0 12px;
    display: flex;
    align-items: center;
    gap: 12px;
    border: none;
    border-radius: 10px;
    background: transparent;
    color: var(--muted);
    font-size: 14px;
    font-weight: 500;
    text-align: left;
  }
  .nav-item:hover:not(:disabled) {
    background: var(--hover);
    color: var(--text);
  }
  .nav-item.active {
    background: var(--surface);
    color: var(--text);
  }
  /* Petite barre d'accent façon Windows 11 */
  .nav-item.active::before {
    content: '';
    position: absolute;
    left: 0;
    top: 50%;
    width: 3px;
    height: 18px;
    margin-top: -9px;
    border-radius: 3px;
    background: var(--accent);
  }
  .nav-icon {
    position: relative;
    display: flex;
    flex: none;
  }
  .dot {
    position: absolute;
    right: -3px;
    top: -3px;
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: var(--accent);
    border: 2px solid var(--sidebar);
  }
  .dot.system {
    box-shadow: 0 0 0 2px color-mix(in srgb, var(--accent) 35%, transparent);
  }
  .banner.info {
    background: color-mix(in srgb, var(--accent) 10%, transparent);
    border-color: color-mix(in srgb, var(--accent) 40%, transparent);
    color: #9be8c6;
  }
  .label {
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .soon {
    margin-left: auto;
    font-size: 11px;
    color: var(--faint);
    background: var(--line-soft);
    border-radius: 999px;
    padding: 2px 8px;
  }

  .sidebar-foot {
    margin-top: auto;
    display: flex;
    flex-direction: column;
    gap: 6px;
  }
  .restore {
    color: var(--warn);
  }
  .restore:disabled {
    opacity: 0.5;
  }
  .version {
    padding: 0 12px;
    font-family: var(--font-mono);
    font-size: 11px;
    color: var(--faint);
  }

  .content {
    overflow-y: auto;
    padding: 28px 32px 32px;
    display: flex;
    flex-direction: column;
    gap: 16px;
  }

  /* Fenêtre étroite : la barre latérale se replie en icônes */
  @media (max-width: 760px) {
    .shell {
      grid-template-columns: 64px minmax(0, 1fr);
    }
    .sidebar {
      padding: 16px 8px;
      align-items: center;
    }
    .brand {
      padding: 4px 0 8px;
      justify-content: center;
    }
    .label {
      display: none;
    }
    .nav-item {
      justify-content: center;
      padding: 0;
      width: 48px;
    }
    .nav {
      align-items: center;
    }
    .content {
      padding: 20px;
    }
  }
</style>
