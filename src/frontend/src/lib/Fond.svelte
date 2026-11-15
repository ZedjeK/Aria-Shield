<script>
  // Fond animé « couloirs » de la zone principale (voir fond-couloirs.js). Le
  // canvas remplit son parent, passe sous les panneaux et ne reçoit aucun clic.
  // Il suit l'état de la protection et les réglages fond.* de la page Réglages.
  import { onMount } from 'svelte'
  import { createFond, SPEEDS } from './fond-couloirs.js'
  import { app, fond } from './state.svelte.js'

  let canvas = $state()
  let moteur = null

  // protege : tunnel et kill switch · dns : résolveur seul · off : rien
  const etat = $derived(app.status?.protected ? 'protege' : app.status?.systemDns ? 'dns' : 'off')

  onMount(() => {
    moteur = createFond(canvas, {
      state: etat,
      active: fond.actif,
      speed: SPEEDS[fond.vitesse] ?? 1,
      min: fond.minimum,
      max: fond.maximum,
    })
    return () => moteur?.destroy()
  })

  // Chaque changement de réglage ou d'état est transmis au moteur.
  $effect(() => {
    moteur?.set({
      state: etat,
      active: fond.actif,
      speed: SPEEDS[fond.vitesse] ?? 1,
      min: fond.minimum,
      max: fond.maximum,
    })
  })
</script>

<canvas bind:this={canvas} class="fond" aria-hidden="true"></canvas>

<style>
  .fond {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
    display: block;
    pointer-events: none;
  }
</style>
