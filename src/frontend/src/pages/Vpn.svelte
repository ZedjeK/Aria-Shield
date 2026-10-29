<script>
  import {
    app, saveProfile, deleteProfile, useProfile, importConf, testVPN, formatBytes, formatAgo,
  } from '../lib/state.svelte.js'

  const profiles = $derived(app.status?.profiles ?? [])
  const active = $derived(app.status?.vpn ?? { configured: false })
  const tunnel = $derived(app.status?.tunnel ?? { up: false, handshakeAgo: -1 })
  const protectedOn = $derived(app.status?.protected ?? false)

  // null = pas de formulaire ; '' = nouveau profil ; sinon l'id du profil modifié
  let editing = $state(null)
  let showKey = $state(false)
  let confirmDelete = $state('')
  let fields = $state({})
  let form = $state(emptyForm())

  const editedProfile = $derived(profiles.find((p) => p.id === editing))
  const locked = (p) => p.active && tunnel.up // profil utilisé par le tunnel ouvert

  function emptyForm() {
    return { name: '', privateKey: '', presharedKey: '', addresses: '', publicKey: '', endpoint: '', keepalive: '25' }
  }

  function startNew() {
    form = emptyForm()
    fields = {}
    showKey = false
    editing = ''
  }

  function startEdit(p) {
    form = {
      name: p.name ?? '',
      privateKey: '',
      presharedKey: '',
      addresses: (p.addresses ?? []).join(', '),
      publicKey: p.publicKey ?? '',
      endpoint: p.endpoint ?? '',
      keepalive: String(p.keepalive ?? 25),
    }
    fields = {}
    showKey = false
    editing = p.id
  }

  async function submit(event) {
    event.preventDefault()
    const res = await saveProfile(editing, { ...form })
    fields = res?.fields ?? {}
    if (!res?.error) {
      editing = null
      form = emptyForm()
    }
  }

  async function remove(p) {
    if (confirmDelete !== p.id) {
      confirmDelete = p.id
      return
    }
    confirmDelete = ''
    await deleteProfile(p.id)
    if (editing === p.id) editing = null
  }

  async function doImport() {
    editing = null
    await importConf()
  }
</script>

<div class="page">
  <div class="page-head">
    <h1>VPN</h1>
    <span class="pill" class:on={tunnel.up}>{tunnel.up ? 'Tunnel ouvert' : 'Tunnel fermé'}</span>
  </div>
  <p class="page-intro">
    Le moteur WireGuard officiel tourne depuis Aria. Importe le fichier <code>.conf</code> de ton fournisseur, ou recopie ses valeurs : tes profils restent chiffrés sur ton PC.
  </p>

  <div class="grid-2">
    <section class="card">
      <div class="card-head">
        <h2>Profils</h2>
        <span class="count">{profiles.length}/20</span>
      </div>

      {#if profiles.length === 0}
        <p class="card-text">Aucun profil pour l'instant. Télécharge une configuration WireGuard chez ton fournisseur (Proton VPN, Mullvad, IVPN…), puis importe-la ici.</p>
      {:else}
        <ul class="profiles">
          {#each profiles as p (p.id)}
            <li class="profile" class:active={p.active}>
              <span class="marker" aria-hidden="true">{p.active ? '▸' : ''}</span>
              <span class="row-text">
                <span class="row-name">{p.name}{#if p.active}<span class="chip ok tag">{tunnel.up ? 'en cours' : 'actif'}</span>{/if}</span>
                <span class="row-detail mono">{p.endpoint}{p.hasPsk ? ' · clé partagée' : ''}</span>
              </span>
              <span class="row-actions">
                {#if !p.active}
                  <button class="btn-ghost tiny" onclick={() => useProfile(p.id)} disabled={app.busy || (tunnel.up && !tunnel.full)}
                    title={protectedOn ? 'Changer de serveur sans couper la protection' : 'Utiliser ce profil à la prochaine activation'}>
                    {protectedOn ? 'basculer' : 'utiliser'}
                  </button>
                {/if}
                <button class="btn-ghost tiny" onclick={() => startEdit(p)} disabled={app.busy || locked(p)} aria-label={`Modifier ${p.name}`}>modifier</button>
                <button class="btn-ghost tiny danger" onclick={() => remove(p)} disabled={app.busy || locked(p)} aria-label={`Supprimer ${p.name}`}>
                  {confirmDelete === p.id ? 'confirmer' : 'supprimer'}
                </button>
              </span>
            </li>
          {/each}
        </ul>
      {/if}

      <div class="actions">
        <button class="btn-primary grow" onclick={doImport} disabled={app.busy || profiles.length >= 20}>importer .conf</button>
        <button class="btn-ghost" onclick={startNew} disabled={app.busy || profiles.length >= 20}>saisie manuelle</button>
      </div>
      {#if protectedOn && profiles.length > 1}
        <p class="hint">« basculer » change de serveur sans couper le kill switch : rien ne sort en clair pendant le changement.</p>
      {:else if active.configured && tunnel.up}
        <p class="hint">Le profil en cours ne se modifie pas tant que le tunnel est ouvert.</p>
      {/if}
      {#if app.status?.userDir}
        <p class="path">Profils chiffrés dans : {app.status.userDir}</p>
      {/if}
    </section>

    {#if editing !== null}
      <section class="card">
        <div class="card-head">
          <h2>{editing ? `Modifier « ${editedProfile?.name ?? ''} »` : 'Nouveau profil'}</h2>
        </div>
        <form class="form" onsubmit={submit}>
          <label class="field">
            <span class="field-label">Nom du profil</span>
            <input bind:value={form.name} placeholder="Ex. Proton Pays-Bas" maxlength="40" autocomplete="off" />
            {#if fields.name}<span class="field-error">{fields.name}</span>{/if}
          </label>

          <span class="eyebrow">Ton côté · section [Interface]</span>
          <label class="field">
            <span class="field-label">Clé privée (PrivateKey)</span>
            <span class="with-btn">
              <input
                type={showKey ? 'text' : 'password'}
                bind:value={form.privateKey}
                placeholder={editing ? '•••••••• (laisse vide pour garder l\'actuelle)' : '44 caractères terminés par ='}
                autocomplete="off"
                spellcheck="false"
              />
              <button type="button" class="btn-ghost small" onclick={() => (showKey = !showKey)} aria-label={showKey ? 'Masquer les clés' : 'Afficher les clés'}>
                {showKey ? 'Masquer' : 'Afficher'}
              </button>
            </span>
            {#if fields.privateKey}<span class="field-error">{fields.privateKey}</span>{/if}
          </label>
          <label class="field">
            <span class="field-label">Adresses (Address)</span>
            <input bind:value={form.addresses} placeholder="Ex. 10.2.0.2/32, fd00::2/128" autocomplete="off" spellcheck="false" />
            {#if fields.addresses}<span class="field-error">{fields.addresses}</span>{/if}
          </label>

          <span class="eyebrow">Le serveur · section [Peer]</span>
          <label class="field">
            <span class="field-label">Clé publique du serveur (PublicKey)</span>
            <input bind:value={form.publicKey} placeholder="44 caractères terminés par =" autocomplete="off" spellcheck="false" />
            {#if fields.publicKey}<span class="field-error">{fields.publicKey}</span>{/if}
          </label>
          <label class="field">
            <span class="field-label">Clé partagée (PresharedKey, facultative)</span>
            <input
              type={showKey ? 'text' : 'password'}
              bind:value={form.presharedKey}
              placeholder={editedProfile?.hasPsk ? '•••••••• (laisse vide pour garder l\'actuelle)' : 'seulement si ton fichier en contient une'}
              autocomplete="off"
              spellcheck="false"
            />
            {#if fields.presharedKey}<span class="field-error">{fields.presharedKey}</span>{/if}
          </label>
          <div class="two">
            <label class="field">
              <span class="field-label">Point d'accès (Endpoint)</span>
              <input bind:value={form.endpoint} placeholder="Ex. 203.0.113.10:51820" autocomplete="off" spellcheck="false" />
              {#if fields.endpoint}<span class="field-error">{fields.endpoint}</span>{/if}
            </label>
            <label class="field">
              <span class="field-label">Keepalive (s)</span>
              <input bind:value={form.keepalive} inputmode="numeric" autocomplete="off" />
              {#if fields.keepalive}<span class="field-error">{fields.keepalive}</span>{/if}
            </label>
          </div>

          <p class="hint">
            Imposé par Aria : tout le trafic passe dans le tunnel (AllowedIPs), et la ligne DNS du fichier est ignorée, car Unbound résout à travers le tunnel.
          </p>

          <div class="actions">
            <button type="submit" class="btn-primary grow" disabled={app.busy}>{app.busy ? 'enregistrement…' : 'enregistrer'}</button>
            <button type="button" class="btn-ghost" onclick={() => (editing = null)} disabled={app.busy}>annuler</button>
          </div>
        </form>
      </section>
    {:else}
      <section class="card">
        <h2>Tunnel</h2>
        <div class="row"><span class="row-name">État</span><span class="value">{tunnel.up ? (tunnel.full ? 'Ouvert, tout le trafic' : 'Ouvert (test)') : 'Fermé'}</span></div>
        <div class="row"><span class="row-name">Serveur</span><span class="value">{active.configured ? active.name : '—'}</span></div>
        <div class="row"><span class="row-name">Kill switch</span><span class="chip {tunnel.killSwitch ? 'ok' : ''}">{tunnel.killSwitch ? 'Actif' : 'Inactif'}</span></div>
        <div class="row"><span class="row-name">Dernière poignée de main</span><span class="value">{tunnel.up ? formatAgo(tunnel.handshakeAgo) : '—'}</span></div>
        <div class="row"><span class="row-name">Reçu / envoyé</span><span class="value">{tunnel.up ? `${formatBytes(tunnel.rxBytes)} / ${formatBytes(tunnel.txBytes)}` : '—'}</span></div>
        <div class="actions">
          <button class="btn-ghost grow" onclick={testVPN} disabled={app.busy || !active.configured || tunnel.up}>
            {app.busy ? 'en cours…' : 'tester la connexion'}
          </button>
        </div>
        {#if !active.configured}
          <p class="hint">Ajoute d'abord un profil.</p>
        {:else if !protectedOn}
          <p class="hint">Ouvre le tunnel du profil actif quelques secondes pour vérifier que le serveur reconnaît ta clé, sans toucher au reste.</p>
        {/if}
      </section>
    {/if}
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
  </section>
</div>

<style>
  .form {
    display: flex;
    flex-direction: column;
    gap: 12px;
  }
  .field {
    display: flex;
    flex-direction: column;
    gap: 6px;
    min-width: 0;
  }
  .field-label {
    font-size: 13px;
    font-weight: 500;
  }
  .field-error {
    font-size: 12px;
    color: var(--danger);
  }
  input {
    width: 100%;
    min-height: 40px;
    padding: 0 12px;
    border-radius: 8px 0;
    corner-shape: bevel;
    border: 1px solid var(--line-strong);
    background: var(--surface-sunken);
    color: var(--text);
    font-family: var(--font-mono);
    font-size: 13px;
  }
  input::placeholder {
    color: var(--faint);
    opacity: 0.6;
    font-style: italic;
  }
  input:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: 1px;
  }
  .with-btn {
    display: flex;
    gap: 8px;
  }
  .small {
    min-height: 40px;
    padding: 0 12px;
    font-size: 12px;
  }
  .two {
    display: grid;
    grid-template-columns: minmax(0, 1fr) 110px;
    gap: 10px;
  }
  .value {
    font-size: 13px;
    color: var(--muted);
    text-align: right;
    overflow-wrap: anywhere;
  }
  .mono {
    font-family: var(--font-mono);
    font-size: 12px;
  }
  .danger {
    color: var(--danger);
  }
  code {
    font-family: var(--font-mono);
    color: var(--text);
  }
  .count {
    font-size: 12px;
    color: var(--faint);
  }
  .profiles {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
  }
  .profile {
    display: grid;
    grid-template-columns: 12px minmax(0, 1fr);
    column-gap: 6px;
    row-gap: 8px;
    padding: 10px 0;
  }
  .profile .row-actions {
    grid-column: 2;
    justify-content: flex-start;
  }
  .profile + .profile {
    border-top: 1px solid var(--line-soft);
  }
  .marker {
    color: var(--action);
    line-height: 1.4;
  }
  .profile.active .row-name {
    color: var(--accent);
  }
  .tag {
    margin-left: 8px;
    vertical-align: 1px;
  }
  .row-actions {
    display: flex;
    gap: 6px;
    flex-wrap: wrap;
    justify-content: flex-end;
  }
  .tiny {
    min-height: 32px;
    padding: 0 10px;
    font-size: 12px;
  }
</style>
