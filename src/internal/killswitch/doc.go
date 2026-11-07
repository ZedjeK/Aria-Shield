// Package killswitch est le pare-feu WFP de wireguard-windows (tunnel/firewall,
// licence MIT, voir COPYING-wireguard-windows.txt), adapté pour Aria : la seule
// modification autorise le processus Aria (chemin de l'exe + utilisateur) au
// lieu du SID du service WireGuard. La session WFP est dynamique : Windows retire
// toutes les règles dès qu'Aria se ferme, même en cas de plantage.
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
