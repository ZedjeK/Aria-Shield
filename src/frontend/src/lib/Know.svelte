<script>
  // Encadré bleu « À savoir » : le commentaire de l'onglet actif, sous les
  // sous-onglets. Fond opaque, pour rester lisible sur le fond animé.
  let { children } = $props()
</script>

<p class="know"><b>À savoir</b>{@render children()}</p>

<style>
  .know {
    margin: 0;
    padding: 12px 14px;
    font-size: 13px;
    line-height: 1.6;
    color: #bdd2f5;
    background: #0a1730;
    border: 1px solid color-mix(in srgb, var(--accent-2) 45%, #0a1730);
    border-left: 3px solid var(--accent-2);
  }
  b {
    margin-right: 8px;
    font-size: 11px;
    font-weight: 600;
    letter-spacing: 1px;
    text-transform: uppercase;
    color: var(--accent-2);
  }
</style>
