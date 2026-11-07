//go:build windows

package toastreg

import (
	"testing"

	"golang.org/x/sys/windows/registry"
)

func TestRemove(t *testing.T) {
	app := "Aria Shield Test toastreg"
	guid := "{12345678-1234-1234-1234-123456789ABC}"
	mk := func(path string) registry.Key {
		k, _, err := registry.CreateKey(registry.CURRENT_USER, path, registry.ALL_ACCESS)
		if err != nil {
			t.Fatal(err)
		}
		return k
	}
	k := mk(aumidPath + app)
	k.SetStringValue(activator, guid)
	k.Close()
	k = mk(clsidPath + guid + `\LocalServer32`)
	k.SetStringValue("", `"C:\x.exe" %1`)
	k.Close()
	mk(appRootPath + app + `\NotificationCategories`).Close()

	if err := Remove(app); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{aumidPath + app, clsidPath + guid, appRootPath + app} {
		if k, err := registry.OpenKey(registry.CURRENT_USER, p, registry.QUERY_VALUE); err == nil {
			k.Close()
			t.Errorf("clé restante : %s", p)
		}
	}
	// Une deuxième fois, sans rien à effacer : pas d'erreur.
	if err := Remove(app); err != nil {
		t.Fatal(err)
	}
}

func TestPointsTo(t *testing.T) {
	path := `Software\Aria Shield Test toastreg pointsTo`
	defer registry.DeleteKey(registry.CURRENT_USER, path)
	for _, c := range []struct {
		value string
		want  bool
	}{
		{`C:\Users\x\Desktop\Aria-Shield.exe`, true},
		{`"C:\Program Files\Aria Shield\Aria-Shield.exe" %1`, true},
		{`C:\Program Files\Autre\autre.exe`, false},
		{`"C:\Aria-Shield.exe.autre\x.exe" %1`, false},
	} {
		k, _, err := registry.CreateKey(registry.CURRENT_USER, path, registry.ALL_ACCESS)
		if err != nil {
			t.Fatal(err)
		}
		k.SetStringValue("", c.value)
		k.Close()
		if got := pointsTo(path, exeName); got != c.want {
			t.Errorf("pointsTo(%q) = %v, attendu %v", c.value, got, c.want)
		}
	}
}

func TestRemoveIgnoresBadGUID(t *testing.T) {
	app := "Aria Shield Test toastreg mauvais GUID"
	k, _, err := registry.CreateKey(registry.CURRENT_USER, aumidPath+app, registry.ALL_ACCESS)
	if err != nil {
		t.Fatal(err)
	}
	// Un GUID trafiqué ne doit jamais mener à effacer CLSID en entier.
	k.SetStringValue(activator, `..\..\`)
	k.Close()
	if err := Remove(app); err != nil {
		t.Fatal(err)
	}
	if k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Classes\CLSID`, registry.QUERY_VALUE); err != nil {
		t.Fatal("CLSID a disparu !")
	} else {
		k.Close()
	}
}
