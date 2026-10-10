<script>
  // Icônes au trait, toutes en 24×24.
  let { name, size = 20 } = $props()
</script>

<svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
  {#if name === 'home'}
    <path d="M4 11l8-7 8 7" /><path d="M6 10v10h12V10" /><path d="M10 20v-5h4v5" />
  {:else if name === 'dns'}
    <rect x="4" y="4" width="16" height="6" rx="2" /><rect x="4" y="14" width="16" height="6" rx="2" /><path d="M8 7h.01" /><path d="M8 17h.01" />
  {:else if name === 'vpn'}
    <rect x="5" y="11" width="14" height="9" rx="2" /><path d="M8 11V8a4 4 0 0 1 8 0v3" />
  {:else if name === 'ghost'}
    <path d="M6 20V10a6 6 0 0 1 12 0v10l-3-2-3 2-3-2z" /><path d="M10 10h.01" /><path d="M14 10h.01" />
  {:else if name === 'tests'}
    <path d="M9 6h11" /><path d="M9 12h11" /><path d="M9 18h11" /><path d="M4 6l1 1 2-2" /><path d="M4 12l1 1 2-2" /><path d="M4 18l1 1 2-2" />
  {:else if name === 'shield'}
    <path d="M12 3l7 3v5c0 4.5-3 8.3-7 10-4-1.7-7-5.5-7-10V6z" />
  {:else if name === 'restore'}
    <path d="M3 12a9 9 0 1 0 3-6.7" /><path d="M3 4v5h5" />
  {:else if name === 'check'}
    <path d="M5 12l5 5 9-10" />
  {:else if name === 'clock'}
    <circle cx="12" cy="12" r="8" /><path d="M12 8v4l2.5 2" />
  {/if}
</svg>
