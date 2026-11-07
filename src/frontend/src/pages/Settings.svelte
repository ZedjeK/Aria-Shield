<script>
  // Page « Réglages » : démarrage avec Windows (tâche planifiée), réactivation
  // de la dernière protection au lancement, et sortie d'Aria. Fermer la
  // fenêtre range Aria dans la zone de notification ; seul « Quitter » l'arrête.
  import { onMount } from 'svelte'
  import Switch from '../lib/Switch.svelte'
  import { app, settings, loadSettings, setAutostart, setResume, quitApp } from '../lib/state.svelte.js'

  onMount(loadSettings)

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
    </section>
  </div>

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
  .quit:hover:not(:disabled) {
    border-color: var(--action);
    color: var(--action);
  }
</style>
