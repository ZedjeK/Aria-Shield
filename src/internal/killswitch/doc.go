// Package killswitch est le pare-feu WFP de wireguard-windows (tunnel/firewall,
// licence MIT, voir COPYING-wireguard-windows.txt), adapté pour Aria : la seule
// modification autorise le processus Aria (chemin de l'exe + utilisateur) au
// lieu du SID du service WireGuard. La session WFP est dynamique : Windows retire
// toutes les règles dès qu'Aria se ferme, même en cas de plantage.
//
// Windows uniquement.
package killswitch
