//go:build windows

package journal

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

// openNoFollow ouvre le journal en ajout, sans suivre de lien.
//
// FILE_FLAG_OPEN_REPARSE_POINT fait ouvrir le lien lui-même plutôt que sa
// cible ; on lit ensuite les attributs sur le fichier réellement ouvert (pas
// sur le chemin, qui pourrait changer entre-temps) et on refuse tout ce qui
// n'est pas un fichier ordinaire. Aucune fenêtre entre la vérification et
// l'ouverture.
func openNoFollow(path string) (*os.File, error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	h, err := windows.CreateFile(p,
		windows.FILE_APPEND_DATA|windows.SYNCHRONIZE|windows.FILE_READ_ATTRIBUTES,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil, windows.OPEN_ALWAYS,
		windows.FILE_ATTRIBUTE_NORMAL|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return nil, err
	}
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(h, &info); err != nil {
		windows.CloseHandle(h)
		return nil, err
	}
	if info.FileAttributes&(windows.FILE_ATTRIBUTE_REPARSE_POINT|windows.FILE_ATTRIBUTE_DIRECTORY) != 0 {
		windows.CloseHandle(h)
		return nil, fmt.Errorf("%s n'est pas un fichier ordinaire (lien ?) : journal désactivé", path)
	}
	return os.NewFile(uintptr(h), path), nil
}
