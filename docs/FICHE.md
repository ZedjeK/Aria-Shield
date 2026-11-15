# Fiche projet : Aria Shield

Ce que cette appli a de propre. À relire avant toute modification, et à tenir à
jour quand l'appli change.

| | |
| --- | --- |
| Nom | Aria Shield (dépôt et exe : `Aria-Shield`) |
| Promesse | Naviguer discrètement en un clic : un résolveur DNS à toi, un VPN avec kill switch et Tor Browser, dans un seul exe portable. |
| Élément orange du logo | l'œil, au centre du « A » glitché ; le logo est délavé quand rien ne protège |
| Fond animé | pas encore posé : la refonte visuelle prévoit un « double sas » en flux continu (lignes qui vont de gauche à droite, un premier sas qui arrête une partie des points hostiles, un second où les autres, ralentis, s'arrêtent) ; la fiche sera complétée à sa livraison |
| Stack | Go 1.25, Wails v3 (bêta) et Svelte 5 ; Unbound et WireGuardNT embarqués |
| Droits administrateur | oui, dès le lancement (manifeste) : DNS des cartes réseau, carte VPN, filtres du pare-feu, Unbound sur le port 53. Tor Browser est lancé avec les droits normaux de l'utilisateur, par un assistant sans élévation. |
| Démarrage automatique | oui, désactivé par défaut : tâche planifiée `\Aria Shield\Démarrage` qui lance une copie de l'exe rangée dans `%ProgramFiles%\Aria Shield`, avec `--silencieux` |
| Installation par l'exe | non : l'exe ne propose pas de s'installer. Il se copie dans `%ProgramFiles%\Aria Shield` seulement quand « Démarrer avec Windows » est activé, et retire cette copie quand il est désactivé. |
| Port local (interface web) | aucun : l'interface est une fenêtre Wails, sans serveur web. Unbound écoute sur `127.0.0.1:53` et `[::1]:53`, jamais ailleurs. |
| Données de l'utilisateur | `%USERPROFILE%\.Aria-Shield` : `reglages.json`, `aria.log`, `vpn-profiles.bin`, `TorBrowser\`, `webview\`, `cache\` (dossier temporaire d'Aria : l'icône des notifications). Secrètes : les profils VPN (`vpn-profiles.bin`, chiffré par DPAPI). |
| Écritures hors du dossier de données | `%ProgramData%\AriaShield` (réservé aux administrateurs : Unbound, sa config, `root.key`, `dns-backup.json`, la liste de blocage, `unbound.log`) ; `%ProgramFiles%\Aria Shield` et la tâche planifiée (démarrage automatique) ; DNS des cartes réseau et la valeur `DisableSmartNameResolution` sous `HKLM\SOFTWARE\Policies\Microsoft\Windows NT\DNSClient`, tous deux rétablis à la désactivation ; carte réseau « AriaShield » et filtres du pare-feu (session dynamique) en mode Discret ; clés de notification sous `HKCU\Software\Classes\AppUserModelId\Aria Shield`, `HKCU\Software\Classes\CLSID\{GUID}` et `HKCU\Software\Aria Shield` (effacées quand on désactive les notifications)  ; rien dans `%TEMP%` : dès son lancement, Aria fait de `.Aria-Shield\cache` son dossier temporaire |
| Connexions sortantes | Unbound : résolution récursive, serveurs racine et serveurs faisant autorité, port 53 (en mode Discret, à travers le tunnel). Serveur WireGuard du profil choisi (UDP). `api.ipify.org` et `api6.ipify.org` : mesure de l'IP publique (activation du mode Discret, page Tests). `api.github.com/repos/ZedjeK/Aria-Shield/releases/latest` : nouvelle version (quelques minutes après le lancement, désactivable ; le téléchargement se fait par toi). `small.oisd.nl` et `big.oisd.nl` : liste de blocage des pubs (à l'activation, puis chaque semaine). `aus1.torproject.org` (index des versions), `dist.torproject.org` et `archive.torproject.org` (téléchargement) et `openpgpkey.torproject.org` (clé de signature) : mode Fantôme seulement. Tor Browser, une fois lancé, utilise le réseau Tor. |
| Composants tiers embarqués | Unbound 1.26.1 (NLnet Labs), WireGuardNT 1.1 (WireGuard LLC, chargé en mémoire depuis l'exe), Tor Browser 15.0.24 (extrait au premier lancement), code dérivé de wireguard-windows (kill switch et pilote), Wails v3, polices Space Grotesk et IBM Plex Mono. Détail : `LICENCES-TIERS.txt`. |
| Réussite de la v0.1 | critère de l'époque non consigné ; critère actuel : « activer la protection » en un clic, vérifié par la page Tests (Windows passe par le résolveur, aucune fuite d'IP, kill switch actif) |

## Écarts assumés au contrat des applis Aria

Ce que cette appli fait autrement que le modèle, avec la raison. Un écart qui n'est
plus justifié se corrige.

- **Journal** : `aria.log`, en texte, au lieu de `journal.json` (les 500 dernières
  opérations). Il raconte aussi ce qu'Aria fait seule (reprise au lancement, relances
  d'Unbound), ce que le format du modèle ne prévoit pas.
- **Dossier système** : une appli élevée doit lancer ses exécutables depuis un dossier
  que seuls les administrateurs modifient. Unbound vit donc dans `%ProgramData%`
  verrouillé, et la copie du démarrage automatique dans `%ProgramFiles%`, et non dans
  `.Aria-Shield\programme`.
- **Livraison** : un exe seul, sans zip ni `fichiers.txt` : Aria ne livre aucun fichier
  à côté de l'exe. La release joint l'exe et son `.sha256`.
- **Mises à jour** : Aria signale une nouvelle version et indique où la télécharger ;
  elle ne la télécharge ni ne l'installe.
- **Version** : le tag est `v` suivi de `ProductVersion` dans `src/build/windows/info.json`
  (`v0.12.0`). Les versions publiées jusqu'à la 0.11 étaient nommées `vX.Y`.
- **Pas encore faits** : installation par l'exe, menu de l'icône complet, refonte visuelle
  et fond animé.
