<script>
  // Page « Réglages » : démarrage avec Windows (tâche planifiée), réactivation
  // de la dernière protection au lancement, et sortie d'Aria. Fermer la
  // fenêtre range Aria dans la zone de notification ; seul « Quitter » l'arrête.
  import { onMount } from 'svelte'
  import Switch from '../lib/Switch.svelte'
  import {
    app, settings, loadSettings, setAutostart, setResume, setNotifications, quitApp,
    update, loadUpdate, checkUpdate, setUpdateCheck,
  } from '../lib/state.svelte.js'

  onMount(() => {
    loadSettings()
    loadUpdate()
  })
  const up = $derived(update.data)

  const d = $derived(settings.data)
  const lastLabel = { aucune: 'aucune', dns: 'DNS (résolveur pour tout Windows)', discret: 'mode Discret (VPN + DNS)' }
  const admin = $derived(app.status?.admin ?? false)
</script>

<div class="page">
  <div class="page-head">
    <h1>Réglages</h1>
  </div>
  <p class="page-intro">
    Fermer la fenêtre ne coupe rien : Aria reste dans la zone de notification, à côté de l'horloge, et ta protection continue. Pour l'arrêter vraiment, utilise « Quitter ».
  </p>

  <div class="grid-2">
    <!-- Démarrage avec Windows et reprise -->
    <section class="card">
      <h2>Démarrage</h2>
      <div class="row">
        <span class="row-text">
          <span class="row-name">Démarrer avec Windows</span>
          <span class="row-detail">À l'ouverture de ta session, dans la zone de notification, sans fenêtre UAC</span>
        </span>
        <Switch
          checked={d?.autostart?.enabled ?? false}
          disabled={!d || app.busy || !admin}
          label="Démarrer Aria Shield avec Windows"
          onchange={setAutostart}
        />
      </div>
      <div class="row">
        <span class="row-text">
          <span class="row-name">Réactiver ma dernière protection</span>
          <span class="row-detail">Au lancement d'Aria · retenue : {d ? lastLabel[d.last] ?? d.last : '…'}</span>
        </span>
        <Switch
          checked={d?.resume ?? false}
          disabled={!d || app.busy}
          label="Réactiver la dernière protection au lancement"
          onchange={setResume}
        />
      </div>
      <div class="row">
        <span class="row-text">
          <span class="row-name">Notifications Windows</span>
          <span class="row-detail">En cas d'incident ou d'échec de la réactivation, et quand tu quittes Aria. Une réactivation réussie reste silencieuse.</span>
        </span>
        <Switch
          checked={d?.notify ?? false}
          disabled={!d || app.busy}
          label="Notifications Windows"
          onchange={setNotifications}
        />
      </div>
      {#if settings.error}
        <p class="error" role="alert">{settings.error}</p>
      {/if}
      <p class="card-text">
        Pour démarrer sans te demander l'autorisation administrateur, Aria crée la tâche planifiée « \Aria Shield\Démarrage » et lance une copie d'elle-même rangée dans Program Files, que seul un administrateur peut modifier. Si tu lances une autre version d'Aria, la copie est remise à jour.
      </p>
      {#if d?.autostart?.enabled && d.autostart.exe}
        <p class="path">Copie lancée au démarrage : {d.autostart.exe}</p>
      {/if}
    </section>

    <!-- Quitter et dossiers -->
    <section class="card">
      <h2>Quitter Aria</h2>
      <p class="card-text">
        Quitter arrête le tunnel, retire le kill switch, rend son DNS d'origine à Windows et arrête le résolveur. Ta dernière protection reste retenue pour le prochain lancement.
      </p>
      <div class="actions">
        <button class="btn-ghost grow quit" onclick={quitApp} disabled={app.busy}>quitter aria shield</button>
      </div>
      <span class="eyebrow">Dossiers</span>
      {#if app.status?.userDir}
        <p class="path">Tes données : {app.status.userDir}</p>
      {/if}
      {#if app.status?.dataDir}
        <p class="path">Dossier système (administrateurs) : {app.status.dataDir}</p>
      {/if}
      {#if app.status?.logPath}
        <p class="path">Journal de ce qu'Aria a fait : {app.status.logPath}</p>
      {/if}
    </section>
  </div>

  <!-- Nouvelles versions -->
  <section class="card">
    <h2>Version</h2>
    <div class="row">
      <span class="row-text">
        <span class="row-name">Vérifier les nouvelles versions</span>
        <span class="row-detail">Quelques minutes après le lancement, une requête à GitHub · rien n'est téléchargé ni installé</span>
      </span>
      <Switch
        checked={up?.enabled ?? false}
        disabled={!up}
        label="Vérifier les nouvelles versions au lancement"
        onchange={setUpdateCheck}
      />
    </div>
    <div class="actions">
      <button class="btn-ghost" onclick={checkUpdate} disabled={update.busy}>
        {update.busy ? 'vérification…' : 'vérifier maintenant'}
      </button>
    </div>
    {#if up?.error}
      <p class="error" role="alert">{up.error}</p>
    {:else if up?.available}
      <p class="card-text">
        La version <strong>{up.latest}</strong> est disponible (tu as la {up.current}). Quitte Aria, puis télécharge le nouvel exe :
      </p>
      <p class="path selectable">{up.url}</p>
    {:else if up?.checked}
      <p class="hint">À jour : la {up.current} est la dernière version publiée.</p>
    {:else}
      <p class="hint">Version installée : {up?.current ?? app.status?.version ?? '…'}.</p>
    {/if}
  </section>

  {#if app.logTitle && (app.log.length || app.actionError)}
    <section class="card">
      <h2>{app.logTitle}</h2>
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
      {/if}
    </section>
  {/if}
</div>

<style>
  .selectable {
    user-select: text;
    font-size: 12px;
    color: var(--accent);
  }
  .quit:hover:not(:disabled) {
    border-color: var(--action);
    color: var(--action);
  }
</style>
