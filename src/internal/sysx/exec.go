// Package sysx regroupe les petits utilitaires système : lancer une commande
// sans fenêtre console et vérifier les droits administrateur.
package sysx

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Run exécute une commande sans ouvrir de fenêtre console et renvoie sa sortie
// combinée (stdout + stderr), nettoyée des espaces en bordure.
func Run(timeout time.Duration, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)
	hideWindow(cmd)

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	err := cmd.Run()
	text := strings.TrimSpace(out.String())
	if ctx.Err() == context.DeadlineExceeded {
		return text, fmt.Errorf("%s : délai dépassé (%s)", name, timeout)
	}
	if err != nil {
		return text, fmt.Errorf("%s : %w", name, err)
	}
	return text, nil
}

// PowerShell exécute un script PowerShell court, sans profil ni fenêtre.
// La sortie est forcée en UTF-8 pour garder les accents des noms de cartes.
func PowerShell(timeout time.Duration, script string) (string, error) {
	script = "[Console]::OutputEncoding = [System.Text.Encoding]::UTF8\n" + script
	return Run(timeout, "powershell.exe",
		"-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass",
		"-Command", script)
}
