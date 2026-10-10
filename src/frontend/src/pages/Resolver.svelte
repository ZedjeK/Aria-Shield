<script>
  import { onMount } from 'svelte'
  import Switch from '../lib/Switch.svelte'
  import {
    app, startDNS, stopDNS, testDNS, enableSystemDNS, disableSystemDNS, blockedReason, chip, loadAdapters,
  } from '../lib/state.svelte.js'

  const running = $derived(app.status?.dnsRunning ?? false)
  const systemDns = $derived(app.status?.systemDns ?? false)
  const adapters = $derived(app.adapters)
  const protectedOn = $derived(app.status?.protected ?? false)

  onMount(loadAdapters)
  const reason = $derived(blockedReason())
  const resolve = $derived(chip(app.test?.resolveOk, app.test === null))
  const dnssec = $derived(chip(app.test?.dnssecOk, app.test === null))

  const switchReason = $derived(
    reason ? reason : !running ? 'Démarre d\'abord le résolveur.' : ''
  )

  function toggleSystem(on) {
    if (on) enableSystemDNS()
    else disableSystemDNS()
  }
</script>

<div class="page">
  <div class="page-head">
    <h1>Résolveur DNS</h1>
    <span class="pill" class:on={running}>{running ? 'Actif' : 'Arrêté'}</span>
  </div>
  <p class="page-intro">
    Unbound tourne depuis Aria, sans rien installer : récursion complète, DNSSEC, aucun DNS tiers. Il s'arrête tout seul quand tu fermes Aria.
  </p>

  <div class="grid-2">
    <section class="card">
      <h2>Résolveur</h2>
      <div class="actions">
        {#if running}
          <button class="btn-ghost grow" onclick={stopDNS} disabled={app.busy || protectedOn}>{app.busy ? 'En cours…' : 'Arrêter'}</button>
        {:else}
          <button class="btn-primary grow" onclick={startDNS} disabled={app.busy || Boolean(reason)}>
            {app.busy ? 'Démarrage…' : 'Démarrer le résolveur'}
          </button>
        {/if}
        <button class="btn-ghost" onclick={testDNS} disabled={app.busy || !running}>Tester</button>
      </div>
      {#if reason}
        <p class="hint">{reason}</p>
      {:else if !running && !app.busy}
        <p class="hint">Démarre le résolveur pour pouvoir le tester.</p>
      {/if}

      <div class="results">
        <div class="row">
          <span class="row-name">Résolution locale</span>
          <span class="chip {resolve.cls}">{resolve.label}</span>
        </div>
        <div class="row">
          <span class="row-name">Validation DNSSEC</span>
          <span class="chip {dnssec.cls}">{dnssec.label}</span>
        </div>
      </div>
    </section>

    <section class="card">
      <div class="card-head">
        <span class="row-text">
          <h2>Utiliser pour tout Windows</h2>
          <span class="row-detail">{systemDns ? 'Toutes tes cartes réseau passent par Unbound' : 'Windows utilise encore le DNS de ta box'}</span>
        </span>
        <Switch
          checked={systemDns}
          disabled={app.busy || protectedOn || (!systemDns && Boolean(switchReason))}
          label="Utiliser le résolveur pour tout Windows"
          onchange={toggleSystem}
        />
      </div>
      {#if protectedOn}
        <p class="hint">Fait partie de la protection active : désactive-la depuis l'Accueil.</p>
      {:else if !systemDns && switchReason}
        <p class="hint">{switchReason}</p>
      {/if}
      <p class="card-text">
        Ta config d'origine est photographiée avant toute modification et remise automatiquement si tu coupes, si Unbound s'arrête ou si tu fermes Aria.
      </p>

      {#if adapters.length}
        <div class="adapters">
          {#each adapters as a (a.alias)}
            <div class="row">
              <span class="row-text">
                <span class="row-name">{a.alias}</span>
                <span class="row-detail mono">{a.servers?.length ? a.servers.join(', ') : 'aucun serveur'}</span>
              </span>
              {#if systemDns}
                <span class="chip {a.onAria ? 'ok' : 'fail'}">{a.onAria ? 'Aria' : 'Hors Aria'}</span>
              {/if}
            </div>
          {/each}
        </div>
      {/if}
    </section>
  </div>

  <section class="card">
    <h2>{app.logTitle || 'Journal'}</h2>
    {#if app.actionError}
      <p class="error" role="alert">{app.actionError}</p>
    {/if}
    {#if app.log.length}
      <ol class="log">
        {#each app.log as s}
          <li class:fail={!s.ok}>
            <span>{s.ok ? '✓' : '✗'}</span>
            <span>{s.label}{s.note ? ` · ${s.note}` : ''}</span>
          </li>
        {/each}
      </ol>
    {:else if !app.actionError}
      <p class="card-text">Le détail de la prochaine action s'affichera ici.</p>
    {/if}
    {#if app.status?.dataDir}
      <p class="path">Dossier de travail : {app.status.dataDir}</p>
    {/if}
  </section>
</div>

<style>
  .results,
  .adapters {
    display: flex;
    flex-direction: column;
    padding-top: 4px;
    border-top: 1px solid var(--line-soft);
  }
  .results .row {
    min-height: 40px;
  }
  .mono {
    font-family: var(--font-mono);
    font-size: 11px;
  }
</style>
