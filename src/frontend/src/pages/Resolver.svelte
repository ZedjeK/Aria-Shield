<script>
  // Page « Résolveur DNS » : démarrer Unbound, le tester, puis l'imposer à
  // tout Windows. L'interrupteur est verrouillé pendant la protection complète,
  // dont le DNS de Windows fait partie.
  import { onMount } from 'svelte'
  import Switch from '../lib/Switch.svelte'
  import {
    app, startDNS, stopDNS, testDNS, enableSystemDNS, disableSystemDNS, blockedReason, chip, loadAdapters,
    ads, loadAds, setAdBlock, updateBlocklist, addAllow, removeAllow,
  } from '../lib/state.svelte.js'

  const running = $derived(app.status?.dnsRunning ?? false)
  const systemDns = $derived(app.status?.systemDns ?? false)
  const adapters = $derived(app.adapters)
  const protectedOn = $derived(app.status?.protected ?? false)

  onMount(() => {
    loadAdapters()
    loadAds()
  })

  // Blocage des pubs
  const ad = $derived(ads.data)
  let newAllow = $state('')
  const listDate = $derived(
    ad?.list?.present ? new Date(ad.list.updated).toLocaleDateString('fr-FR', { day: 'numeric', month: 'long', year: 'numeric' }) : '',
  )
  const listCount = $derived(ad?.list?.present ? ad.list.count.toLocaleString('fr-FR') : '')

  async function submitAllow(event) {
    event.preventDefault()
    const domain = newAllow.trim()
    if (!domain) return
    if (await addAllow(domain)) newAllow = ''
  }
  const reason = $derived(blockedReason())
  const resolve = $derived(chip(app.test?.resolveOk, app.test === null))
  const dnssec = $derived(chip(app.test?.dnssecOk, app.test === null))

  // Pourquoi l'interrupteur est grisé (affiché sous lui), "" s'il est utilisable.
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
    Unbound tourne depuis Aria, sans rien installer : récursion complète, DNSSEC, aucun DNS tiers. Il continue quand tu fermes la fenêtre (Aria reste dans la zone de notification) et s'arrête quand tu quittes Aria.
  </p>

  <div class="grid-2">
    <!-- Unbound : démarrage, arrêt et résultats des deux tests -->
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

    <!-- Bascule du DNS de Windows, avec l'état de chaque carte réseau -->
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
        Ta config d'origine est photographiée avant toute modification et remise automatiquement si tu coupes, si Unbound s'arrête ou si tu quittes Aria.
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

  <!-- Blocage des pubs et des traceurs, avec les exceptions -->
  <section class="card">
    <div class="card-head">
      <span class="row-text">
        <h2>Bloquer les pubs et les traceurs</h2>
        <span class="row-detail">
          {#if ad?.active}
            Actif : {listCount} domaines renvoyés vers 0.0.0.0
          {:else if ad?.enabled}
            Activé : s'appliquera au démarrage du résolveur
          {:else}
            Désactivé : le résolveur répond pour tous les domaines
          {/if}
        </span>
      </span>
      <Switch
        checked={ad?.enabled ?? false}
        disabled={!ad || app.busy || Boolean(reason)}
        label="Bloquer les pubs et les traceurs"
        onchange={setAdBlock}
      />
    </div>
    {#if ads.error}
      <p class="error" role="alert">{ads.error}</p>
    {/if}
    <p class="card-text">
      Aria télécharge la liste {ad?.source ?? 'OISD small'}, n'en garde que des noms de domaine valides et écrit elle-même les règles : une liste piégée ne peut rien rediriger, seulement bloquer. Activer, couper ou mettre à jour redémarre le résolveur (une à deux secondes sans résolution). Les pubs servies depuis le domaine du site lui-même ne sont pas bloquées.
    </p>
    <div class="actions">
      <button class="btn-ghost" onclick={updateBlocklist} disabled={!ad || app.busy || Boolean(reason)}>
        {app.busy ? 'En cours…' : 'mettre à jour la liste'}
      </button>
    </div>
    {#if listDate}
      <p class="hint">Liste du {listDate} · mise à jour automatique chaque semaine, au lancement d'Aria.</p>
    {/if}

    <div class="allow">
      <span class="eyebrow">Exceptions</span>
      <form class="allow-form" onsubmit={submitAllow}>
        <input
          bind:value={newAllow}
          placeholder="Ex. suivi.exemple.fr"
          aria-label="Domaine à ne pas bloquer"
          autocomplete="off"
          spellcheck="false"
          maxlength="253"
        />
        <button type="submit" class="btn-ghost" disabled={!ad || app.busy || !newAllow.trim()}>débloquer</button>
      </form>
      {#if ad?.allow?.length}
        {#each ad.allow as domain (domain)}
          <div class="row">
            <span class="row-name mono">{domain}</span>
            <button class="btn-ghost tiny" onclick={() => removeAllow(domain)} disabled={app.busy} aria-label={`Retirer l'exception ${domain}`}>retirer</button>
          </div>
        {/each}
      {:else}
        <p class="hint">Un site ou un lien ne marche plus ? Ajoute son domaine ici : lui et ses sous-domaines ne seront plus bloqués.</p>
      {/if}
    </div>
  </section>

  <!-- Journal de la dernière action -->
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
      <p class="path">Dossier système (réservé aux administrateurs) : {app.status.dataDir}</p>
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
  .allow {
    display: flex;
    flex-direction: column;
    gap: 8px;
    padding-top: 12px;
    border-top: 1px solid var(--line-soft);
  }
  .allow-form {
    display: flex;
    gap: 8px;
  }
  .allow-form input {
    flex: 1;
    min-width: 0;
    min-height: 44px;
    padding: 0 12px;
    border-radius: 8px 0;
    corner-shape: bevel;
    border: 1px solid var(--line-strong);
    background: var(--surface-sunken);
    color: var(--text);
    font-family: var(--font-mono);
    font-size: 13px;
  }
  .allow-form input::placeholder {
    color: var(--faint);
    opacity: 0.6;
    font-style: italic;
  }
  .allow-form input:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: 1px;
  }
  .allow .row .mono {
    font-size: 12px;
    overflow-wrap: anywhere;
  }
  .tiny {
    min-height: 32px;
    padding: 0 10px;
    font-size: 12px;
  }
</style>
