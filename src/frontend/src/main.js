// Point d'entrée de l'interface : charge les polices et le thème, puis monte
// l'application Svelte dans <div id="app"> (index.html).
import { mount } from 'svelte'

// Polices embarquées dans l'exe : aucune requête vers Google Fonts au lancement.
import '@fontsource/space-grotesk/500.css'
import '@fontsource/space-grotesk/600.css'
import '@fontsource/space-grotesk/700.css'
import '@fontsource/ibm-plex-mono/400.css'
import '@fontsource/ibm-plex-mono/500.css'
import '@fontsource/ibm-plex-mono/600.css'

import './app.css'
import App from './App.svelte'

mount(App, { target: document.getElementById('app') })
