<script>
  // Page « Réglages » : démarrage avec Windows (tâche planifiée), réactivation
  // de la dernière protection au lancement, et sortie d'Aria. Fermer la
  // fenêtre range Aria dans la zone de notification ; seul « Quitter » l'arrête.
  import { onMount } from 'svelte'
  import Switch from '../lib/Switch.svelte'
  import Subtabs from '../lib/Subtabs.svelte'
  import Know from '../lib/Know.svelte'
  import {
    app, settings, loadSettings, setAutostart, setResume, setNotifications, quitApp,
    update, loadUpdate, checkUpdate, setUpdateCheck,
    fond, setFond, FOND_MIN, FOND_MAX,
  } from '../lib/state.svelte.js'

  onMount(() => {
    loadSettings()
    loadUpdate()
  })
  const up = $derived(update.data)

  const d = $derived(settings.data)
  const lastLabel = { aucune: 'aucune', dns: 'DNS (résolveur pour tout Windows)', discret: 'mode Discret (VPN + DNS)' }
  const admin = $derived(app.status?.admin ?? false)

  // Sous-onglets de la page.
  const tabs = [
    { id: 'demarrage', label: 'Démarrage' },
    { id: 'fond', label: 'Fond animé' },
    { id: 'version', label: 'Version' },
  ]
  let tab = $state('demarrage')

  // Vitesse du fond : un choix exclusif parmi trois.
  const vitesses = [
    { id: 'lente', label: 'lente', detail: 'Paquets plus lents, fond plus calme.' },
    { id: 'normale', label: 'normale', detail: 'Le rythme par défaut.' },
    { id: 'rapide', label: 'rapide', detail: 'Paquets plus vifs, fond plus agité.' },
  ]
</script>

<div class="page">
  <div class="page-head">
    <h1>Réglages</h1>
  </div>
  <Subtabs {tabs} bind:active={tab} />

  {#if tab === 'demarrage'}
    <Know>
      Fermer la fenêtre ne coupe rien : Aria reste dans la zone de notification, à côté de l'horloge, et ta protection continue. Pour l'arrêter vraiment, utilise « Quitter ».
    </Know>

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

  {:else if tab === 'fond'}
    <Know>
      Le fond raconte ce que fait Aria. Les points cyan et bleus sont tes connexions ; les rouges, ce qu'Aria arrête. Premier sas : le résolveur (pubs, traceurs). Second sas : le kill switch. Il se met en pause quand la fenêtre est cachée et s'arrête si Windows demande moins d'animations.
    </Know>

  <!-- Fond animé de la zone principale -->
  <section class="card">
    <h2>Fond animé</h2>
    <div class="row">
      <span class="row-text">
        <span class="row-name">Afficher le fond animé</span>
        <span class="row-detail">Il se met en pause quand la fenêtre est cachée, et ne bouge plus si Windows demande moins d'animations.</span>
      </span>
      <Switch checked={fond.actif} label="Afficher le fond animé" onchange={(on) => setFond({ actif: on })} />
    </div>
    <span class="eyebrow">Vitesse</span>
    <div class="opts" role="radiogroup" aria-label="Vitesse du fond">
      {#each vitesses as v (v.id)}
        <button
          class="opt"
          class:on={fond.vitesse === v.id}
          role="radio"
          aria-checked={fond.vitesse === v.id}
          disabled={!fond.actif}
          onclick={() => setFond({ vitesse: v.id })}
        >
          <span class="ot">{v.label}</span>
          <span class="ob">{v.detail}</span>
        </button>
      {/each}
    </div>
    <label class="range">
      <span class="eyebrow">Points à l'écran, au minimum</span>
      <span class="range-line">
        <input
          type="range"
          min={FOND_MIN[0]}
          max={FOND_MIN[1]}
          value={fond.minimum}
          disabled={!fond.actif}
          oninput={(e) => setFond({ minimum: Number(e.currentTarget.value) })}
        />
        <output>{fond.minimum}</output>
      </span>
    </label>
    <label class="range">
      <span class="eyebrow">Points à l'écran, au maximum</span>
      <span class="range-line">
        <input
          type="range"
          min={FOND_MAX[0]}
          max={FOND_MAX[1]}
          value={fond.maximum}
          disabled={!fond.actif}
          oninput={(e) => setFond({ maximum: Number(e.currentTarget.value) })}
        />
        <output>{fond.maximum}</output>
      </span>
    </label>
    {#if fond.error}
      <p class="error" role="alert">{fond.error}</p>
    {/if}
  </section>

  {:else}
    <Know>
      La vérification contacte api.github.com, qui voit ton adresse IP, ou celle du VPN en mode Discret. Rien n'est téléchargé ni installé.
    </Know>

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
  {/if}

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
  .range {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }
  .range-line {
    display: flex;
    align-items: center;
    gap: 12px;
  }
  .range input {
    flex: 1;
    min-width: 0;
    accent-color: var(--accent);
  }
  .range output {
    min-width: 2.5ch;
    text-align: right;
    font-family: var(--font-mono);
    font-size: 13px;
    color: var(--text);
  }
  .quit:hover:not(:disabled) {
    border-color: var(--action);
    color: var(--action);
  }
</style>
