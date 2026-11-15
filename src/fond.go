package main

// Réglages du fond animé de la zone principale : actif ou non, vitesse, nombre
// de points. Ce sont les clés fond.actif, fond.vitesse, fond.minimum et
// fond.maximum de reglages.json. Le dessin est entièrement côté interface ;
// le Go ne fait que garder les réglages, bornés (prefs.Fond.Normalize).

import "aria-shield/internal/prefs"

// FondReglages est ce que l'interface lit et renvoie.
type FondReglages struct {
	Actif   bool   `json:"actif"`
	Vitesse string `json:"vitesse"` // lente, normale ou rapide
	Minimum int    `json:"minimum"`
	Maximum int    `json:"maximum"`
	Error   string `json:"error,omitempty"`
}

func fondReglages(f prefs.Fond) FondReglages {
	return FondReglages{Actif: f.Actif, Vitesse: f.Vitesse, Minimum: f.Minimum, Maximum: f.Maximum}
}

// Fond lit les réglages du fond animé.
func (s *ShieldService) Fond() FondReglages {
	return fondReglages(s.prefs.Load().Fond)
}

// SetFond enregistre les réglages du fond animé et renvoie les valeurs
// retenues (ramenées aux bornes permises). Rien n'est écrit dans le journal :
// ce n'est pas une action sur le système.
func (s *ShieldService) SetFond(actif bool, vitesse string, minimum, maximum int) FondReglages {
	f := prefs.Fond{Actif: actif, Vitesse: vitesse, Minimum: minimum, Maximum: maximum}.Normalize()
	if err := s.prefs.Update(func(p *prefs.Prefs) { p.Fond = f }); err != nil {
		res := fondReglages(s.prefs.Load().Fond)
		res.Error = "réglage non enregistré : " + err.Error()
		return res
	}
	return fondReglages(f)
}
