// Faux runtime Wails pour les captures d'écran (voir vite.captures.config.js) :
// aucun événement ne vient jamais du Go dans un navigateur ordinaire.
export const Events = {
  On() {
    return () => {}
  },
  Emit() {
    return Promise.resolve(true)
  },
}
