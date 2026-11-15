// Package tempdir fait du dossier « cache » des données d'Aria son dossier
// temporaire, pour que rien ne soit écrit dans %TEMP%.
//
// Le contrat des applis Aria interdit %TEMP% : tout vit dans
// %USERPROFILE%\.Aria-<Nom>. Or le service de notifications de Wails dépose
// l'icône des notifications dans le dossier temporaire du processus
// (os.TempDir) : il faut donc changer ce dossier avant qu'il démarre.
//
// Aria écrit ici en administrateur, dans un dossier de l'utilisateur : Redirect
// refuse un « cache » qui serait un lien (jonction, lien symbolique) posé par
// un autre programme pour la faire écrire ailleurs.
package tempdir

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Redirect fait pointer le dossier temporaire du processus sur
// <dataDir>\cache, créé si besoin, et renvoie l'ancien dossier temporaire.
// En cas d'échec, rien n'est changé.
func Redirect(dataDir string) (previous string, err error) {
	previous = os.TempDir()
	if dataDir == "" {
		return previous, fmt.Errorf("pas de dossier de données")
	}
	cache := filepath.Join(dataDir, "cache")
	if err := os.MkdirAll(cache, 0o755); err != nil {
		return previous, err
	}
	// Lstat ne suit pas le lien : un lien symbolique ou une jonction
	// (ModeIrregular depuis Go 1.23) n'est pas un vrai dossier.
	st, err := os.Lstat(cache)
	if err != nil {
		return previous, err
	}
	if !st.IsDir() || st.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 {
		return previous, fmt.Errorf("%s n'est pas un vrai dossier (lien ?)", cache)
	}
	// TMP et TEMP pour Windows, TMPDIR ailleurs (tests).
	for _, name := range []string{"TMP", "TEMP", "TMPDIR"} {
		if err := os.Setenv(name, cache); err != nil {
			return previous, err
		}
	}
	return previous, nil
}

// RemoveOld efface dans l'ancien dossier temporaire les fichiers dont le nom
// correspond à pattern (syntaxe de filepath.Match), sans suivre de lien : seuls
// les fichiers ordinaires partent. Renvoie le nombre de fichiers effacés.
func RemoveOld(dir, pattern string) int {
	if dir == "" {
		return 0
	}
	matches, err := filepath.Glob(filepath.Join(dir, escape(pattern)))
	if err != nil {
		return 0
	}
	n := 0
	for _, m := range matches {
		if st, err := os.Lstat(m); err != nil || !st.Mode().IsRegular() {
			continue
		}
		if os.Remove(m) == nil {
			n++
		}
	}
	return n
}

// escape protège les crochets, qui sont spéciaux dans filepath.Match mais pas
// dans les noms d'icône. Les accolades n'ont rien de spécial.
func escape(pattern string) string {
	return strings.NewReplacer("[", `\[`, "]", `\]`).Replace(pattern)
}
