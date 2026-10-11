//go:build windows

package paths

import "golang.org/x/sys/windows"

// lockDown réserve le dossier aux administrateurs et au système : propriétaire
// Administrateurs, héritage coupé, contrôle total pour SYSTEM et Administrateurs,
// aucun droit pour les autres. Les ACE héritables sont propagées aux fichiers
// déjà présents. Un programme sans droits admin ne peut donc ni y déposer ni y
// remplacer un exécutable qu'Aria lancerait en administrateur.
func lockDown(dir string) error {
	// D:P = DACL protégée (pas d'héritage du parent) ;
	// OICI = héritée par les sous-dossiers et fichiers ; FA = contrôle total.
	sd, err := windows.SecurityDescriptorFromString("O:BAD:P(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)")
	if err != nil {
		return err
	}
	owner, _, err := sd.Owner()
	if err != nil {
		return err
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		return err
	}
	return windows.SetNamedSecurityInfo(dir, windows.SE_FILE_OBJECT,
		windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
		owner, nil, dacl, nil)
}
