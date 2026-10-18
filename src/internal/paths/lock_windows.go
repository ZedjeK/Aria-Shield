//go:build windows

package paths

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// D:P = DACL protégée (pas d'héritage du parent) ; OICI = héritée par les
// sous-dossiers et fichiers ; FA = contrôle total. Propriétaire : Administrateurs.
const lockedSDDL = "O:BAD:P(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)"

// prepareSystemDir crée le dossier système déjà verrouillé, ou vérifie qu'un
// dossier existant a bien été créé par Aria avant de le reverrouiller.
//
// Un utilisateur standard peut créer des dossiers dans ProgramData : un programme
// sans droits admin pourrait créer celui d'Aria avant elle (ou en faire une
// jonction) et y déposer une DLL qu'Unbound chargerait en administrateur. Un
// dossier qui n'appartient ni aux Administrateurs ni au système, ou qui est un
// lien, est donc mis de côté (renommé, jamais supprimé) et remplacé par un neuf.
// Renvoie le nom du dossier mis de côté, ou "".
func prepareSystemDir(dir string) (string, error) {
	sd, err := windows.SecurityDescriptorFromString(lockedSDDL)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
		return "", err
	}
	err = createLocked(dir, sd)
	if err == nil {
		return "", nil
	}
	if !errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
		return "", err
	}
	if ownedByAria(dir) {
		return "", lockDown(dir, sd)
	}
	aside := fmt.Sprintf("%s.suspect-%s", dir, time.Now().Format("20060102-150405"))
	if err := os.Rename(dir, aside); err != nil {
		return "", fmt.Errorf("dossier %s suspect et impossible à écarter : %w", dir, err)
	}
	return aside, createLocked(dir, sd)
}

// createLocked crée le dossier avec son descripteur final dès la création :
// aucun instant où il hériterait des droits permissifs de ProgramData.
func createLocked(dir string, sd *windows.SECURITY_DESCRIPTOR) error {
	p, err := windows.UTF16PtrFromString(dir)
	if err != nil {
		return err
	}
	sa := windows.SecurityAttributes{SecurityDescriptor: sd}
	sa.Length = uint32(unsafe.Sizeof(sa))
	return windows.CreateDirectory(p, &sa)
}

// ownedByAria : un vrai dossier (pas une jonction ni un lien) dont le propriétaire
// est Administrateurs ou SYSTEM. Sans droits admin, impossible de s'attribuer
// ces propriétaires : seul Aria (ou un administrateur) a pu le créer.
func ownedByAria(dir string) bool {
	p, err := windows.UTF16PtrFromString(dir)
	if err != nil {
		return false
	}
	attrs, err := windows.GetFileAttributes(p)
	if err != nil || attrs&windows.FILE_ATTRIBUTE_DIRECTORY == 0 || attrs&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		return false
	}
	sd, err := windows.GetNamedSecurityInfo(dir, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION)
	if err != nil {
		return false
	}
	owner, _, err := sd.Owner()
	if err != nil || owner == nil {
		return false
	}
	return owner.IsWellKnown(windows.WinBuiltinAdministratorsSid) || owner.IsWellKnown(windows.WinLocalSystemSid)
}

// lockDown réapplique propriétaire et DACL : contrôle total pour SYSTEM et
// Administrateurs, aucun droit pour les autres, héritage coupé. Les ACE
// héritables sont propagées aux fichiers déjà présents.
func lockDown(dir string, sd *windows.SECURITY_DESCRIPTOR) error {
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
