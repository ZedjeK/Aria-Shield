// Package autostart lance Aria à l'ouverture de la session Windows, sans
// fenêtre UAC, par une tâche planifiée « \Aria Shield\Démarrage ».
//
// Pourquoi une tâche et pas la clé Run : Aria exige les droits administrateur,
// et Windows ignore sans rien dire les programmes de la clé Run qui demandent
// l'élévation. Une tâche « avec les privilèges les plus élevés » démarre Aria
// directement en administrateur.
//
// Pourquoi une copie dans Program Files : la tâche lance l'exe en admin sans
// rien demander. S'il restait sur le Bureau, n'importe quel programme sans
// droits pourrait le remplacer et obtenir l'administrateur à la prochaine
// ouverture de session. Program Files n'est modifiable que par les
// administrateurs ; la tâche lance donc la copie qui s'y trouve, et Aria la
// remet à jour quand tu lances une autre version.
package autostart

// TaskName est le nom complet de la tâche planifiée.
const TaskName = `\Aria Shield\Démarrage`

// SilentArg est l'argument passé par la tâche : Aria démarre dans la zone de
// notification, sans ouvrir sa fenêtre.
const SilentArg = "--silencieux"

// State décrit le démarrage automatique, pour l'interface.
type State struct {
	Enabled bool   `json:"enabled"`
	Exe     string `json:"exe"` // copie lancée par la tâche
}
