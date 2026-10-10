<script>
  import { onMount } from 'svelte'
  import Icon from '../lib/Icon.svelte'
  import {
    app, tor, refreshTor, launchTor, updateAndLaunchTor, launchTorAnyway, checkTorUpdate, formatBytes,
  } from '../lib/state.svelte.js'

  const st = $derived(tor.state)
  const progress = $derived(st?.progress)
  const pct = $derived(progress?.total > 0 ? Math.round((progress.done / progress.total) * 100) : null)
  const protectedOn = $derived(app.status?.protected ?? false)
  const upd = $derived(tor.update)

  onMount(refreshTor)
</script>

<div class="page">
  <div class="page-head">
    <h1>Mode Fantôme</h1>
    <span class="pill" class:on={st?.running}>{st?.running ? 'Tor Browser ouvert' : st?.installed ? 'Prêt' : 'Intégré'}</span>
  </div>
  <p class="page-intro">
    Tor Browser fait rebondir ta navigation sur trois relais du réseau Tor : aucun site, ni ton FAI, ni le VPN ne peut relier ce que tu fais à ton IP.
  </p>

  <div class="grid-2">
    <section class="card">
      <div class="launch-head">
        <span class="ghost-icon"><Icon name="ghost" size={26} /></span>
        <span class="row-text">
          <h2>Tor Browser</h2>
          <span class="row-detail">
            {#if st?.installed}Version {st.version}{:else}Version {upd?.current ?? '15.0.24'} intégrée, extraite au premier lancement{/if}
          </span>
        </span>
      </div>

      <button class="cta" onclick={launchTor} disabled={tor.busy || st?.running || !app.status?.admin}>
        {#if tor.busy}En cours…{:else if st?.running}Déjà ouvert{:else}Lancer Tor Browser{/if}
      </button>

      {#if progress?.active}
        <div class="progress" role="status">
          <span class="row-detail">{progress.phase}{pct !== null ? ` · ${pct} %` : ''}</span>
          <div class="bar"><div class="fill" class:indeterminate={pct === null} style:width={pct !== null ? `${pct}%` : '40%'}></div></div>
          {#if progress.total > 0}
            <span class="row-detail">{formatBytes(progress.done)} sur {formatBytes(progress.total)}</span>
          {/if}
        </div>
      {/if}

      {#if tor.prompt && upd}
        <div class="notice" class:warn={!upd.available}>
          {#if upd.available}
            <p class="notice-title">Mise à jour disponible : {upd.current} → {upd.latest}</p>
            <p class="card-text">Les versions anciennes de Tor Browser peuvent contenir des failles corrigées depuis. La mise à jour garde tes favoris, réglages et ponts.</p>
            <div class="actions">
              <button class="btn-primary grow" onclick={updateAndLaunchTor} disabled={tor.busy}>Mettre à jour et lancer</button>
              <button class="btn-ghost" onclick={launchTorAnyway} disabled={tor.busy}>Lancer quand même</button>
            </div>
          {:else}
            <p class="notice-title">Impossible de vérifier les mises à jour</p>
            <p class="card-text">{upd.error}. Tor Browser vérifiera lui-même une fois connecté.</p>
            <div class="actions">
              <button class="btn-primary grow" onclick={launchTorAnyway} disabled={tor.busy}>Lancer</button>
            </div>
          {/if}
        </div>
      {/if}

      {#if tor.error}
        <p class="error" role="alert">{tor.error}</p>
      {/if}

      <div class="actions">
        <button class="btn-ghost grow" onclick={checkTorUpdate} disabled={tor.busy}>Vérifier les mises à jour</button>
      </div>
      {#if upd?.checked && !upd.available && !tor.prompt}
        <p class="hint">À jour : {upd.latest} est la dernière version publiée.</p>
      {/if}
    </section>

    <section class="card">
      <h2>Pour bien l'utiliser</h2>
      <div class="row">
        <span class="row-text">
          <span class="row-name">Mode Discret actif</span>
          <span class="row-detail">{protectedOn ? 'Tor passe par le VPN : ton FAI ne voit même pas que tu utilises Tor' : 'Recommandé : sinon ton FAI voit que tu te connectes à Tor'}</span>
        </span>
        <span class="chip {protectedOn ? 'ok' : 'warn'}">{protectedOn ? 'Oui' : 'Non'}</span>
      </div>
      <div class="row">
        <span class="row-text">
          <span class="row-name">Lancé sans droits admin</span>
          <span class="row-detail">Aria ouvre Tor Browser avec tes droits normaux, jamais en administrateur</span>
        </span>
        <span class="chip ok">Oui</span>
      </div>
      <p class="card-text">
        Seul ce que tu fais <strong>dans Tor Browser</strong> est anonyme. Ne t'y connecte pas à tes comptes personnels, n'y installe pas d'extensions et n'agrandis pas la fenêtre : ce sont les trois façons classiques de se faire reconnaître.
      </p>
    </section>
  </div>

  {#if tor.log.length}
    <section class="card">
      <h2>Journal</h2>
      <ol class="log">
        {#each tor.log as s}
          <li class:fail={!s.ok}>
            <span>{s.ok ? '✓' : '✗'}</span>
            <span>{s.label}{s.note ? ` · ${s.note}` : ''}</span>
          </li>
        {/each}
      </ol>
    </section>
  {/if}
</div>

<style>
  .launch-head {
    display: flex;
    align-items: center;
    gap: 14px;
  }
  .ghost-icon {
    width: 52px;
    height: 52px;
    flex: none;
    border-radius: 14px;
    display: flex;
    align-items: center;
    justify-content: center;
    color: var(--ghost);
    background: color-mix(in srgb, var(--ghost) 14%, transparent);
  }
  .cta {
    width: 100%;
    min-height: 52px;
    border-radius: 14px;
    font-size: 16px;
    font-weight: 600;
    background: var(--ghost);
    border: 1.5px solid var(--ghost);
    color: #160f2b;
  }
  .cta:disabled {
    opacity: 0.45;
  }
  .progress {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }
  .bar {
    height: 8px;
    border-radius: 999px;
    background: var(--line-soft);
    overflow: hidden;
  }
  .fill {
    height: 100%;
    border-radius: 999px;
    background: var(--ghost);
    transition: width 0.3s;
  }
  .fill.indeterminate {
    animation: slide 1.2s ease-in-out infinite;
  }
  @keyframes slide {
    from {
      transform: translateX(-100%);
    }
    to {
      transform: translateX(250%);
    }
  }
  .notice {
    padding: 14px;
    border-radius: 12px;
    border: 1px solid color-mix(in srgb, var(--ghost) 45%, transparent);
    background: color-mix(in srgb, var(--ghost) 8%, transparent);
    display: flex;
    flex-direction: column;
    gap: 8px;
  }
  .notice.warn {
    border-color: rgba(245, 165, 36, 0.45);
    background: rgba(245, 165, 36, 0.08);
  }
  .notice .actions {
    flex-direction: column;
  }
  .notice-title {
    margin: 0;
    font-weight: 600;
    font-size: 14px;
  }
  :global(.chip.warn) {
    background: rgba(245, 165, 36, 0.14);
    color: var(--warn);
  }
</style>
