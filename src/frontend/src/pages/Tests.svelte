<script>
  import { app, testAll, chip } from '../lib/state.svelte.js'

  const running = $derived(app.status?.dnsRunning ?? false)
  const systemDns = $derived(app.status?.systemDns ?? false)
  const t = $derived(app.sysTests)

  const resolve = $derived(chip(app.test?.resolveOk, app.test === null))
  const dnssec = $derived(chip(app.test?.dnssecOk, app.test === null))
  const marker = $derived(chip(t?.markerOk, !t || !systemDns))
  const cards = $derived(chip(t?.adaptersOk, !t || !systemDns))
  const browsers = $derived(t?.browsers ?? [])
  const protectedOn = $derived(app.status?.protected ?? false)
  const leak = $derived(t?.leak ?? app.status?.leak)
  const v4 = $derived(chip(leak?.v4Ok, !protectedOn || !leak))
  const v6 = $derived(chip(leak?.v6Ok, !protectedOn || !leak))
  const ks = $derived(app.status?.tunnel?.killSwitch ?? false)
</script>

<div class="page">
  <div class="page-head">
    <h1>Tests</h1>
    <button class="btn-ghost" onclick={testAll} disabled={app.busy}>
      {app.busy ? 'En cours…' : 'Relancer'}
    </button>
  </div>
  <p class="page-intro">Vérifie que ce qu'Aria annonce est vrai.</p>

  <div class="grid-2">
    <section class="card">
      <h2>Résolveur</h2>
      <div class="row">
        <span class="row-text">
          <span class="row-name">Résolution locale</span>
          <span class="row-detail">Unbound répond sur 127.0.0.1</span>
        </span>
        <span class="chip {resolve.cls}">{resolve.label}</span>
      </div>
      <div class="row">
        <span class="row-text">
          <span class="row-name">Validation DNSSEC</span>
          <span class="row-detail">Une signature cassée doit être rejetée</span>
        </span>
        <span class="chip {dnssec.cls}">{dnssec.label}</span>
      </div>
      {#if app.test?.detail}
        <p class="card-text">{app.test.detail}</p>
      {/if}
      {#if !running}
        <p class="hint">Démarre le résolveur depuis la page Résolveur DNS.</p>
      {/if}
    </section>

    <section class="card">
      <h2>Windows</h2>
      <div class="row">
        <span class="row-text">
          <span class="row-name">Windows passe par Aria</span>
          <span class="row-detail">{t?.markerDetail || 'Un nom connu du seul résolveur d\'Aria'}</span>
        </span>
        <span class="chip {marker.cls}">{marker.label}</span>
      </div>
      <div class="row">
        <span class="row-text">
          <span class="row-name">Cartes réseau</span>
          <span class="row-detail">{t?.adaptersDetail || 'Toutes doivent pointer sur le résolveur'}</span>
        </span>
        <span class="chip {cards.cls}">{cards.label}</span>
      </div>
      {#if !systemDns}
        <p class="hint">Active « Utiliser pour tout Windows » sur la page Résolveur DNS.</p>
      {/if}
    </section>
  </div>

  <section class="card">
    <h2>Fuites d'IP</h2>
    <div class="row">
      <span class="row-text">
        <span class="row-name">IPv4 publique</span>
        <span class="row-detail">
          {#if leak?.after?.v4}Vue par les sites : {leak.after.v4}{leak.before?.v4 ? ` · ta box : ${leak.before.v4}` : ''}{:else}Mesurée via ipify.org quand la protection est active{/if}
        </span>
      </span>
      <span class="chip {v4.cls}">{v4.label}</span>
    </div>
    <div class="row">
      <span class="row-text">
        <span class="row-name">IPv6 publique</span>
        <span class="row-detail">
          {#if protectedOn && leak}{leak.after?.v6 ? `Vue par les sites : ${leak.after.v6}` : 'Aucune IPv6 ne sort : rien à fuir'}{:else}Ton adresse IPv6 d'origine ne doit jamais apparaître{/if}
        </span>
      </span>
      <span class="chip {v6.cls}">{v6.label}</span>
    </div>
    <div class="row">
      <span class="row-text">
        <span class="row-name">Kill switch</span>
        <span class="row-detail">Bloque tout ce qui ne passe pas par le tunnel</span>
      </span>
      <span class="chip {ks ? 'ok' : 'idle'}">{ks ? 'Actif' : 'Inactif'}</span>
    </div>
    {#if !protectedOn}
      <p class="hint">Active la protection depuis l'Accueil pour mesurer les fuites.</p>
    {:else if leak?.detail}
      <p class="card-text">{leak.detail}</p>
    {/if}
  </section>

  <section class="card">
    <h2>Navigateurs</h2>
    <p class="card-text">
      Certains navigateurs ont leur propre « DNS sécurisé » qui ignore Windows, et donc Aria. Aria se contente de le signaler : rien n'est modifié dans tes navigateurs.
    </p>
    {#if !t}
      <p class="hint">Clique sur « Relancer » pour inspecter tes navigateurs.</p>
    {:else if browsers.length === 0}
      <p class="hint">Aucun navigateur reconnu trouvé.</p>
    {:else}
      {#each browsers as b (b.name)}
        <div class="row">
          <span class="row-text">
            <span class="row-name">{b.name}</span>
            <span class="row-detail">{b.mode}{b.policy ? ' · imposé par stratégie' : ''}</span>
          </span>
          <span class="chip {b.bypass ? 'warn' : 'ok'}">{b.bypass ? 'Contourne' : 'OK'}</span>
        </div>
      {/each}
      {#if browsers.some((b) => b.bypass)}
        <p class="hint">
          Pour qu'un navigateur passe par Aria, tape « DNS » dans la recherche de ses paramètres et désactive l'option « DNS sécurisé » (ou « DNS via HTTPS » dans Firefox).
        </p>
      {/if}
    {/if}
  </section>
</div>

<style>
  :global(.chip.warn) {
    background: rgba(245, 165, 36, 0.14);
    color: var(--warn);
  }
</style>
