// Package killswitch est le pare-feu WFP de wireguard-windows (tunnel/firewall,
// licence MIT, voir COPYING-wireguard-windows.txt), adapté pour Aria :
//
//   - il autorise le processus Aria (chemin de l'exe + utilisateur) au lieu du
//     SID du service WireGuard ;
//   - addFilter (blocker.go) ajoute à chaque filtre les réglages du verrou
//     persistant, et les sorties par le tunnel peuvent être autorisées « en
//     dur » (SetHardTunnel) ;
//   - lock_windows.go ajoute le verrou persistant (voir ce fichier).
//
// La session du kill switch est dynamique : Windows retire ses règles dès
// qu'Aria se ferme, même en cas de plantage. Le verrou, lui, reste posé.
//
// Ce que laisse passer le kill switch (rules.go), tout le reste étant bloqué :
//
//   - le trafic de la carte « Aria » (le tunnel lui-même) ;
//   - le processus Aria, pour joindre le serveur VPN hors du tunnel ;
//   - la boucle locale (127.0.0.1 et ::1), donc le résolveur Unbound ;
//   - DHCP et NDP, pour que la connexion à la box ne tombe pas ;
//   - les cartes virtuelles Hyper-V.
//
// Le code est volontairement laissé tel que l'écrit WireGuard (commentaires
// anglais compris) : plus facile à comparer avec l'original et à mettre à jour.
//
// Windows uniquement.
package killswitch
