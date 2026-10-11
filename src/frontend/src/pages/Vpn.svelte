<script>
  import {
    app, saveProfile, deleteProfile, testVPN, formatBytes, formatAgo,
  } from '../lib/state.svelte.js'

  const profile = $derived(app.status?.vpn ?? { configured: false })
  const tunnel = $derived(app.status?.tunnel ?? { up: false, handshakeAgo: -1 })
  const protectedOn = $derived(app.status?.protected ?? false)

  let editing = $state(false)
  let showKey = $state(false)
  let confirmDelete = $state(false)
  let fields = $state({})
  let form = $state({ name: '', privateKey: '', addresses: '', publicKey: '', endpoint: '', keepalive: '25' })

  const showForm = $derived(!profile.configured || editing)

  function startEdit() {
    form = {
      name: profile.name ?? '',
      privateKey: '',
      addresses: (profile.addresses ?? []).join(', '),
      publicKey: profile.publicKey ?? '',
      endpoint: profile.endpoint ?? '',
      keepalive: String(profile.keepalive ?? 25),
    }
    fields = {}
    editing = true
  }

  async function submit(event) {
    event.preventDefault()
    const res = await saveProfile({ ...form })
    fields = res?.fields ?? {}
    if (!res?.error) {
      editing = false
      showKey = false
      form.privateKey = ''
    }
  }

  async function remove() {
    if (!confirmDelete) {
      confirmDelete = true
      return
    }
    confirmDelete = false
    await deleteProfile()
    editing = false
  }
</script>

<div class="page">
  <div class="page-head">
    <h1>VPN</h1>
    <span class="pill" class:on={tunnel.up}>{tunnel.up ? 'Tunnel ouvert' : 'Tunnel fermé'}</span>
  </div>
  <p class="page-intro">
    Le moteur WireGuard officiel tourne depuis Aria. Saisis ici les valeurs du fichier de configuration de ton fournisseur : elles restent chiffrées sur ton PC.
  </p>

  <div class="grid-2">
    <section class="card">
      <div class="card-head">
        <h2>Configuration VPN</h2>
        {#if profile.configured && !editing}
          <span class="pill on">Enregistrée</span>
        {/if}
      </div>

      {#if showForm}
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
                placeholder={profile.configured ? '•••••••• (laisse vide pour garder l\'actuelle)' : '44 caractères terminés par ='}
                autocomplete="off"
                spellcheck="false"
              />
              <button type="button" class="btn-ghost small" onclick={() => (showKey = !showKey)} aria-label={showKey ? 'Masquer la clé' : 'Afficher la clé'}>
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
            <button type="submit" class="btn-primary grow" disabled={app.busy}>{app.busy ? 'Enregistrement…' : 'Enregistrer'}</button>
            {#if editing}
              <button type="button" class="btn-ghost" onclick={() => (editing = false)} disabled={app.busy}>Annuler</button>
            {/if}
          </div>
        </form>
      {:else}
        <div class="row"><span class="row-name">Nom</span><span class="value">{profile.name}</span></div>
        <div class="row"><span class="row-name">Clé privée</span><span class="value mono">••••••••</span></div>
        <div class="row"><span class="row-name">Adresses</span><span class="value mono">{profile.addresses?.join(', ')}</span></div>
        <div class="row"><span class="row-name">Clé du serveur</span><span class="value mono">{profile.publicKey?.slice(0, 10)}…</span></div>
        <div class="row"><span class="row-name">Point d'accès</span><span class="value mono">{profile.endpoint}</span></div>
        <div class="row"><span class="row-name">Keepalive</span><span class="value">{profile.keepalive} s</span></div>
        <div class="actions">
          <button class="btn-ghost grow" onclick={startEdit} disabled={app.busy || tunnel.up}>Modifier</button>
          <button class="btn-ghost danger" onclick={remove} disabled={app.busy || tunnel.up}>
            {confirmDelete ? 'Confirmer la suppression' : 'Supprimer'}
          </button>
        </div>
        {#if tunnel.up}
          <p class="hint">Désactive la protection pour modifier ou supprimer le profil.</p>
        {/if}
        {#if app.status?.userDir}
          <p class="path">Profil chiffré dans : {app.status.userDir}</p>
        {/if}
      {/if}
    </section>

    <section class="card">
      <h2>Tunnel</h2>
      <div class="row"><span class="row-name">État</span><span class="value">{tunnel.up ? (tunnel.full ? 'Ouvert, tout le trafic' : 'Ouvert (test)') : 'Fermé'}</span></div>
      <div class="row"><span class="row-name">Kill switch</span><span class="chip {tunnel.killSwitch ? 'ok' : ''}">{tunnel.killSwitch ? 'Actif' : 'Inactif'}</span></div>
      <div class="row"><span class="row-name">Dernière poignée de main</span><span class="value">{tunnel.up ? formatAgo(tunnel.handshakeAgo) : '—'}</span></div>
      <div class="row"><span class="row-name">Reçu / envoyé</span><span class="value">{tunnel.up ? `${formatBytes(tunnel.rxBytes)} / ${formatBytes(tunnel.txBytes)}` : '—'}</span></div>
      <div class="actions">
        <button class="btn-ghost grow" onclick={testVPN} disabled={app.busy || !profile.configured || tunnel.up}>
          {app.busy ? 'En cours…' : 'Tester la connexion'}
        </button>
      </div>
      {#if !profile.configured}
        <p class="hint">Enregistre d'abord ta configuration.</p>
      {:else if !protectedOn}
        <p class="hint">Ouvre le tunnel quelques secondes pour vérifier que le serveur reconnaît ta clé, sans toucher au reste.</p>
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
</style>
