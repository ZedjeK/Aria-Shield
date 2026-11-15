// Fond animé « couloirs » de la zone principale : un double sas, en flux continu.
//
// Des paquets partent de la gauche et vont vers la droite, sur des couloirs
// horizontaux. Les paquets cyan, bleus et blancs sont les connexions saines :
// ils traversent les deux sas. Les rouges sont ce qu'Aria arrête : le premier
// sas (le résolveur) en stoppe une partie ; les autres ralentissent entre les
// deux sas, dans la zone de quarantaine, et s'arrêtent au second (le kill
// switch).
//
// Le dessin suit l'état de la protection :
//   protege : les deux sas ;
//   dns     : le premier seulement, les rouges qui le passent traversent ;
//   off     : aucun sas, les rouges sortent à droite.
//
// Tout reste dans le canvas : pas de réseau, rien d'écrit. Le fond se met en
// pause quand la fenêtre est cachée, et ne bouge plus si Windows demande moins
// d'animations (une image fixe est alors dessinée).

const CFG = {
  vmin: 70, // vitesse des paquets, en pixels par seconde
  vmax: 140,
  badRate: 0.3, // part de paquets rouges
  stopAtFirst: 0.55, // part des rouges arrêtés par le premier sas
  laneGap: 64, // écart visé entre deux couloirs
  maxLanes: 11,
  spawnRate: 5,
  fxLife: 0.8,
  gates: [0.4, 0.72], // position des sas, en part de la largeur
}

const COL = {
  cyan: '#7df9ff',
  blue: '#5c8bff',
  white: '#ffffff',
  red: '#ff5c7a',
  amber: '#ffd166',
  faint: '#8197b8',
}

// Facteur de vitesse de chaque réglage.
export const SPEEDS = { lente: 0.6, normale: 1, rapide: 1.7 }

const rnd = (a, b) => a + Math.random() * (b - a)
const pick = (a) => a[Math.floor(Math.random() * a.length)]
const ease = (t) => t * t * (3 - 2 * t)

function labelAt(cx, x, y, text, color, size) {
  cx.save()
  cx.font = `500 ${size}px "IBM Plex Mono", monospace`
  cx.fillStyle = color
  cx.textAlign = 'center'
  cx.fillText(text, x, y)
  cx.restore()
}

/**
 * Crée le fond dans un canvas. Renvoie { set, destroy } :
 *   set({ state, active, speed, min, max }) change un ou plusieurs réglages ;
 *   destroy() arrête tout et retire les écouteurs.
 */
export function createFond(canvas, initial = {}) {
  const cx = canvas.getContext('2d')
  const S = { active: true, speed: 1, min: 10, max: 26, state: 'protege', ...initial }
  const motion = matchMedia('(prefers-reduced-motion: reduce)')

  let W = 0
  let H = 0
  let lanes = []
  let P = [] // paquets
  let FX = [] // éclats
  let heat = [[], []] // cellules allumées de chaque sas
  let target = 0
  let retarget = 0
  let last = 0
  let raf = 0
  let uid = 0
  let clock = 0
  let destroyed = false
  const cnt = { g1: 0, g2: 0 }

  const gx = (i) => W * CFG.gates[i]
  const gateOn = (i) => S.state === 'protege' || (S.state === 'dns' && i === 0)
  const animated = () => S.active && !motion.matches && !document.hidden && !destroyed

  function resize() {
    const r = canvas.getBoundingClientRect()
    const dpr = Math.min(window.devicePixelRatio || 1, 2)
    W = r.width
    H = r.height
    canvas.width = Math.max(1, Math.round(W * dpr))
    canvas.height = Math.max(1, Math.round(H * dpr))
    cx.setTransform(dpr, 0, 0, dpr, 0, 0)
    const n = Math.max(5, Math.min(CFG.maxLanes, Math.floor(H / CFG.laneGap)))
    lanes = Array.from({ length: n }, (_, i) => ((i + 0.5) * H) / n)
    heat = [new Array(n).fill(0), new Array(n).fill(0)]
    P = P.filter((p) => p.lane < n)
    draw()
  }

  function spawn(x0) {
    const bad = Math.random() < CFG.badRate
    const v0 = rnd(CFG.vmin, CFG.vmax)
    const lane = Math.floor(Math.random() * lanes.length)
    P.push({
      id: ++uid,
      lane,
      x: x0 === undefined ? -14 : x0,
      y: lanes[lane],
      kind: bad ? 'bad' : 'good',
      c: bad ? 'red' : pick(['cyan', 'cyan', 'blue', 'white']),
      v0,
      v: v0,
      stop1: bad && Math.random() < CFG.stopAtFirst,
      slowed: false,
    })
  }

  function burst(x, y, gate, lane) {
    FX.push({ t: 0, life: CFG.fxLife, x, y, type: 'burst' })
    if (gate >= 0 && heat[gate]) heat[gate][lane] = 1
  }

  function update(dt) {
    clock += dt
    const x1 = gx(0)
    const x2 = gx(1)
    for (let i = P.length - 1; i >= 0; i--) {
      const p = P[i]
      p.slowed = false
      if (p.kind === 'bad') {
        // Entre les deux sas, un rouge ralentit jusqu'à presque s'arrêter.
        if (p.x >= x1 && p.x < x2 && gateOn(1)) {
          p.v = p.v0 * (1 - 0.9 * ease((p.x - x1) / (x2 - x1)))
          p.slowed = true
        } else {
          p.v = p.v0
        }
      }
      const nx = p.x + p.v * S.speed * dt
      if (p.x < x1 && nx >= x1) {
        if (p.kind === 'bad' && gateOn(0) && p.stop1) {
          burst(x1, p.y, 0, p.lane)
          cnt.g1++
          P.splice(i, 1)
          continue
        }
        if (p.kind === 'good' && gateOn(0)) heat[0][p.lane] = Math.max(heat[0][p.lane], 0.45)
      }
      if (p.x < x2 && nx >= x2) {
        if (p.kind === 'bad' && gateOn(1)) {
          burst(x2, p.y, 1, p.lane)
          cnt.g2++
          P.splice(i, 1)
          continue
        }
        if (p.kind === 'good' && gateOn(1)) heat[1][p.lane] = Math.max(heat[1][p.lane], 0.45)
      }
      p.x = nx
      if (p.x > W + 24) {
        if (p.kind === 'bad') FX.push({ t: 0, life: 0.9, x: W, y: p.y, type: 'leak' })
        P.splice(i, 1)
      }
    }
    for (let i = FX.length - 1; i >= 0; i--) {
      FX[i].t += dt
      if (FX[i].t > FX[i].life) FX.splice(i, 1)
    }
    for (const g of heat) for (let k = 0; k < g.length; k++) g[k] = Math.max(0, g[k] - dt * 1.6)
    if (clock > retarget) {
      target = Math.round(rnd(S.min, S.max))
      retarget = clock + rnd(2.5, 6)
    }
    if (P.length < target && Math.random() < dt * CFG.spawnRate * S.speed) spawn()
  }

  function draw() {
    cx.clearRect(0, 0, W, H)
    if (!S.active || W === 0) return
    const step = lanes.length > 1 ? lanes[1] - lanes[0] : 60
    const x1 = gx(0)
    const x2 = gx(1)

    // Zone entre les sas : teinte ambre, « quarantaine », quand le second sas tient.
    if (gateOn(1)) {
      cx.fillStyle = 'rgba(255,209,102,.045)'
      cx.fillRect(x1, 0, x2 - x1, H)
      cx.save()
      cx.strokeStyle = 'rgba(255,209,102,.07)'
      cx.lineWidth = 1
      for (let x = x1 - H; x < x2; x += 14) {
        const a = Math.max(x1, x)
        const b = Math.min(x2, x + H)
        cx.beginPath()
        cx.moveTo(a, Math.max(0, H - (a - x)))
        cx.lineTo(b, Math.max(0, H - (b - x)))
        cx.stroke()
      }
      cx.restore()
      labelAt(cx, (x1 + x2) / 2, 22, 'QUARANTAINE', 'rgba(255,209,102,.75)', 10)
    }

    // Couloirs
    cx.save()
    cx.strokeStyle = 'rgba(39,64,107,.55)'
    cx.lineWidth = 1
    for (const y of lanes) {
      cx.beginPath()
      cx.moveTo(0, y)
      cx.lineTo(W, y)
      cx.stroke()
    }
    cx.fillStyle = 'rgba(39,64,107,.8)'
    for (const y of lanes) for (let x = 48; x < W; x += 96) cx.fillRect(x, y - 2, 1, 4)
    cx.restore()

    // Sas : une cellule par couloir, qui s'allume quand un paquet la touche.
    for (let i = 0; i < 2; i++) {
      const x = gx(i)
      const on = gateOn(i)
      const h = step * 0.7
      lanes.forEach((y, k) => {
        const hot = heat[i][k] ?? 0
        cx.save()
        cx.fillStyle = on ? `rgba(125,249,255,${0.18 + 0.6 * hot})` : 'rgba(129,151,184,.10)'
        cx.strokeStyle = on ? `rgba(125,249,255,${0.6 + 0.4 * hot})` : 'rgba(129,151,184,.28)'
        cx.lineWidth = 1
        if (on && hot > 0.05) {
          cx.shadowColor = COL.cyan
          cx.shadowBlur = 14 * hot
        }
        cx.beginPath()
        cx.moveTo(x - 4, y - h / 2)
        cx.lineTo(x + 4, y - h / 2 + 4)
        cx.lineTo(x + 4, y + h / 2)
        cx.lineTo(x - 4, y + h / 2 - 4)
        cx.closePath()
        cx.fill()
        cx.stroke()
        cx.restore()
      })
      labelAt(cx, x, H - 14, `${i ? 'KILL SWITCH' : 'RÉSOLVEUR'} · ${i ? cnt.g2 : cnt.g1}`, on ? COL.cyan : COL.faint, 10)
    }

    // Paquets : petits carrés biseautés
    for (const p of P) {
      const bad = p.kind === 'bad'
      const z = bad ? 6 : 4.5
      const col = bad ? (p.slowed ? '#ff9a7a' : COL.red) : COL[p.c]
      cx.save()
      cx.fillStyle = col
      cx.shadowColor = col
      cx.shadowBlur = bad ? 10 : 6
      cx.beginPath()
      cx.moveTo(p.x - z / 2, p.y - z / 2)
      cx.lineTo(p.x + z / 2 - 1.5, p.y - z / 2)
      cx.lineTo(p.x + z / 2, p.y - z / 2 + 1.5)
      cx.lineTo(p.x + z / 2, p.y + z / 2)
      cx.lineTo(p.x - z / 2 + 1.5, p.y + z / 2)
      cx.lineTo(p.x - z / 2, p.y + z / 2 - 1.5)
      cx.closePath()
      cx.fill()
      cx.shadowBlur = 0
      cx.globalAlpha = 0.35
      cx.fillRect(p.x - z / 2 - 9, p.y - 0.5, 9, 1)
      cx.restore()
    }

    // Éclats : un rouge arrêté, ou un rouge qui sort à droite
    for (const f of FX) {
      const k = f.t / f.life
      cx.save()
      cx.globalAlpha = 1 - k
      if (f.type === 'burst') {
        cx.fillStyle = COL.red
        for (let a = 0; a < 7; a++) {
          const t = a * 0.9 + 0.3
          const r = 4 + 24 * k
          cx.fillRect(f.x + Math.cos(t) * r - 1.5, f.y + Math.sin(t) * r - 1.5, 3, 3)
        }
      } else {
        cx.fillStyle = 'rgba(255,92,122,.5)'
        cx.fillRect(W - 4, f.y - 12, 4, 24)
      }
      cx.restore()
    }
  }

  function frame(ts) {
    raf = 0
    if (!animated()) return
    const dt = Math.min(0.05, last ? (ts - last) / 1000 : 0.016)
    last = ts
    update(dt)
    draw()
    raf = requestAnimationFrame(frame)
  }
  function start() {
    if (!raf && animated()) {
      last = 0
      raf = requestAnimationFrame(frame)
    }
  }
  function stop() {
    if (raf) cancelAnimationFrame(raf)
    raf = 0
  }

  // Image de départ : des paquets déjà en route, dont un rouge ralenti entre les sas.
  function seed() {
    P = []
    FX = []
    const n = Math.round((S.min + S.max) / 2)
    for (let i = 0; i < n; i++) spawn(rnd(0, W))
    for (const p of P) if (p.kind === 'bad' && gateOn(0) && p.x > gx(0) && p.stop1) p.x = rnd(gx(1) * 0.6, gx(1) - 6)
  }

  const onVisibility = () => (document.hidden ? stop() : start())
  const onMotion = () => {
    if (motion.matches) {
      stop()
      draw()
    } else {
      start()
    }
  }
  document.addEventListener('visibilitychange', onVisibility)
  motion.addEventListener('change', onMotion)
  const ro = new ResizeObserver(() => resize())
  ro.observe(canvas)

  resize()
  seed()
  draw()
  start()

  return {
    set(o) {
      const was = S.state
      Object.assign(S, o)
      if (o.state && o.state !== was) cnt.g1 = cnt.g2 = 0
      if ('active' in o) {
        if (o.active) {
          if (!P.length) seed()
          start()
        } else {
          stop()
        }
      }
      if ('min' in o || 'max' in o) retarget = 0
      if (!raf) draw()
    },
    destroy() {
      destroyed = true
      stop()
      ro.disconnect()
      document.removeEventListener('visibilitychange', onVisibility)
      motion.removeEventListener('change', onMotion)
    },
  }
}
