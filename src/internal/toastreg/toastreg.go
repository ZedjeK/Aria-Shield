// Package toastreg efface ce que les notifications Windows inscrivent dans le
// registre de l'utilisateur, quand tu les désactives.
//
// Pour afficher ses notifications, le service de Wails inscrit l'appli sous
// HKCU : HKCU\Software\Classes\AppUserModelId\<nom> (avec le GUID de
// l'activateur), HKCU\Software\Classes\CLSID\<GUID> (l'exe à relancer quand on
// clique sur une notification) et HKCU\Software\<nom> (catégories). Il dépose
// aussi l'icône dans %TEMP%. Remove retire tout cela.
package toastreg
