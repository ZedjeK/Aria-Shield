<script>
  // Interrupteur façon Windows 11.
  let { checked = false, disabled = false, label, onchange } = $props()
</script>

<button
  class="switch"
  class:on={checked}
  role="switch"
  aria-checked={checked}
  aria-label={label}
  {disabled}
  onclick={() => onchange?.(!checked)}
>
  <span class="knob"></span>
</button>

<style>
  .switch {
    position: relative;
    flex: none;
    width: 46px;
    height: 24px;
    padding: 0;
    border-radius: 999px;
    border: 1.5px solid var(--faint);
    background: transparent;
    transition: background 0.15s, border-color 0.15s;
  }
  /* zone de clic confortable sans agrandir le dessin */
  .switch::after {
    content: '';
    position: absolute;
    inset: -10px -4px;
  }
  .knob {
    position: absolute;
    top: 50%;
    left: 4px;
    width: 12px;
    height: 12px;
    margin-top: -6px;
    border-radius: 50%;
    background: var(--faint);
    transition: transform 0.15s, background 0.15s;
  }
  .switch.on {
    background: var(--accent);
    border-color: var(--accent);
  }
  .switch.on .knob {
    transform: translateX(22px);
    background: #04121c;
  }
  .switch:disabled {
    opacity: 0.4;
  }
</style>
