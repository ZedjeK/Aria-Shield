//go:build windows

package autostart

import (
	"strings"
	"testing"
	"unicode/utf16"
)

func TestTaskXML(t *testing.T) {
	x := taskXML("S-1-5-21-1-2-3-1001", `C:\Program Files\Aria Shield\Aria-Shield.exe`)
	for _, want := range []string{
		"<RunLevel>HighestAvailable</RunLevel>",
		"<ExecutionTimeLimit>PT0S</ExecutionTimeLimit>",
		"<UserId>S-1-5-21-1-2-3-1001</UserId>",
		`<Command>"C:\Program Files\Aria Shield\Aria-Shield.exe"</Command>`,
		"<Arguments>--silencieux</Arguments>",
	} {
		if !strings.Contains(x, want) {
			t.Errorf("XML sans %q", want)
		}
	}
	// Un chemin avec des caractères XML reste échappé.
	if x := taskXML("S-1", `C:\a&b<c>\x.exe`); !strings.Contains(x, `C:\a&amp;b&lt;c&gt;\x.exe`) {
		t.Errorf("chemin non échappé : %s", x)
	}
}

func TestUTF16File(t *testing.T) {
	b := utf16File("é")
	if b[0] != 0xFF || b[1] != 0xFE {
		t.Fatal("BOM absent")
	}
	u := []uint16{uint16(b[2]) | uint16(b[3])<<8}
	if string(utf16.Decode(u)) != "é" {
		t.Fatalf("encodage : % x", b)
	}
}

func TestSamePath(t *testing.T) {
	if !samePath(`C:\Program Files\Aria Shield\Aria-Shield.exe`, `c:\program files\aria shield\.\Aria-Shield.exe`) {
		t.Error("chemins identiques non reconnus")
	}
	if samePath(`C:\a\Aria-Shield.exe`, `C:\b\Aria-Shield.exe`) {
		t.Error("chemins différents confondus")
	}
}
