<script>
  // Sous-onglets d'une page, en haut de la zone principale. Forme en encoche
  // (coin haut droit coupé) ; l'onglet actif a le contour cyan et une barre
  // épaisse dessous. Flèches gauche et droite pour passer d'un onglet à l'autre.
  //   tabs   : [{ id, label }]
  //   active : id de l'onglet affiché (lié au parent)
  let { tabs, active = $bindable(), label = 'Sections de la page' } = $props()

  function key(e, i) {
    let j = -1
    if (e.key === 'ArrowRight') j = (i + 1) % tabs.length
    else if (e.key === 'ArrowLeft') j = (i + tabs.length - 1) % tabs.length
    if (j < 0) return
    e.preventDefault()
    active = tabs[j].id
    e.currentTarget.parentElement.children[j]?.focus()
  }
</script>

<div class="tabs" role="tablist" aria-label={label}>
  {#each tabs as t, i (t.id)}
    <button
      class="tab"
      class:on={active === t.id}
      role="tab"
      aria-selected={active === t.id}
      tabindex={active === t.id ? 0 : -1}
      onclick={() => (active = t.id)}
      onkeydown={(e) => key(e, i)}
    >
      <span>{t.label}</span>
    </button>
  {/each}
</div>

<style>
  .tabs {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
    border-bottom: 1px solid var(--line);
  }
  .tab {
    position: relative;
    border: 0;
    padding: 1px 1px 0;
    background: var(--line-strong);
    clip-path: polygon(0 0, calc(100% - 12px) 0, 100% 12px, 100% 100%, 0 100%);
  }
  .tab span {
    display: block;
    padding: 10px 20px 11px;
    background: var(--bg);
    clip-path: polygon(0 0, calc(100% - 11px) 0, 100% 11px, 100% 100%, 0 100%);
    font: 500 12px var(--font-mono);
    letter-spacing: 0.5px;
    color: var(--muted);
    text-transform: uppercase;
  }
  .tab:hover span {
    background: var(--hover);
    color: var(--text);
  }
  .tab.on {
    background: var(--accent);
  }
  .tab.on span {
    background: #07182a;
    color: var(--accent);
  }
  .tab.on::after {
    content: '';
    position: absolute;
    left: 0;
    right: 0;
    bottom: 0;
    height: 3px;
    background: var(--accent);
  }
  .tab:focus-visible {
    outline: 2px solid var(--action);
    outline-offset: 2px;
  }
</style>
