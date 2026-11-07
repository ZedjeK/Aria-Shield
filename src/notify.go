package main

// Notifications Windows d'Aria, pour ce que tu dois savoir même fenêtre
// cachée : un incident (Unbound arrêté, protection coupée, réactivation
// impossible au lancement), la sortie d'Aria, et à chaque fermeture de la
// fenêtre le rappel « Aria continue dans la zone de notification ». Une
// réactivation réussie au démarrage reste silencieuse.
//
// Le service de Wails inscrit l'appli dans le registre de l'utilisateur. Rien
// n'y est écrit tant que les notifications sont désactivées, et les désactiver
// efface ces clés (internal/toastreg).

import (
	"context"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/services/notifications"

	"aria-shield/internal/prefs"
	"aria-shield/internal/toastreg"
)

// appName : nom de l'appli dans Wails, donc dans le registre des notifications.
const appName = "Aria Shield"

// notifier enveloppe le service de Wails : un échec d'inscription auprès de
// Windows ne doit jamais empêcher Aria de démarrer, seulement couper les
// notifications.
type notifier struct {
	ns    *notifications.NotificationService
	prefs *prefs.Store

	mu      sync.Mutex
	ctx     context.Context
	opts    application.ServiceOptions
	started bool // le service de Wails a démarré dans ce processus
	cleaned bool // ses clés ont été effacées depuis (réglage désactivé)
	seq     atomic.Int64
}

func newNotifier(p *prefs.Store) *notifier {
	return &notifier{ns: notifications.New(), prefs: p}
}

// ServiceStartup : inscription auprès de Windows, seulement si les
// notifications sont activées.
func (n *notifier) ServiceStartup(ctx context.Context, opts application.ServiceOptions) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.ctx, n.opts = ctx, opts
	if n.prefs.Load().Notify {
		n.startLocked()
	}
	return nil
}

func (n *notifier) startLocked() {
	if n.started {
		return
	}
	n.started = n.ns.ServiceStartup(n.ctx, n.opts) == nil
}

// ServiceShutdown : le service de Wails réenregistre ses catégories en
// partant ; on l'en dispense si les clés ont été effacées.
func (n *notifier) ServiceShutdown() error {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.started && !n.cleaned {
		n.ns.ServiceShutdown()
	}
	return nil
}

// send affiche une notification, si elles sont activées.
func (n *notifier) send(title, body string) {
	n.mu.Lock()
	ready := n.started && !n.cleaned
	n.mu.Unlock()
	if !ready || !n.prefs.Load().Notify {
		return
	}
	n.ns.SendNotification(notifications.NotificationOptions{
		ID:    "aria-shield-" + strconv.FormatInt(n.seq.Add(1), 10),
		Title: title,
		Body:  body,
	})
}

// setEnabled applique le réglage. Désactiver efface les clés du registre ;
// réactiver après un effacement prend effet au prochain lancement (Windows
// garde l'ancien activateur jusque-là). Renvoie une note pour le journal.
func (n *notifier) setEnabled(on bool) (string, error) {
	if err := n.prefs.Update(func(p *prefs.Prefs) { p.Notify = on }); err != nil {
		return "", err
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	if !on {
		n.cleaned = true
		return "inscription retirée du registre", toastreg.Remove(appName)
	}
	if n.cleaned {
		return "actives au prochain lancement d'Aria", nil
	}
	n.startLocked()
	if !n.started {
		return "Windows a refusé l'inscription : réessaie au prochain lancement", nil
	}
	return "", nil
}

// trayHint rappelle, à chaque fermeture de la fenêtre, qu'Aria continue dans
// la zone de notification.
func (n *notifier) trayHint() {
	n.send("Aria Shield continue en arrière-plan",
		"Ta protection reste active. Pour rouvrir Aria ou la quitter, utilise son icône dans la zone de notification.")
}
