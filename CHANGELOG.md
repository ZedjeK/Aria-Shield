# Journal des versions

Un bloc par version, le plus récent en haut. Les notes complètes d'une version
sont dans `docs/releases/`.

## Non publié

Rien pour l'instant.

## 0.12.0 (2026-11-15)

### Ajouté

- Verrou au démarrage (désactivé par défaut) : en mode Discret, des règles de pare-feu persistantes bloquent tout hors du tunnel, du démarrage du PC jusqu'à la remontée du tunnel et après un plantage d'Aria. Commande de secours `Aria-Shield.exe --deverrouiller`.
- Fond animé « couloirs » dans la zone principale : un double sas en flux continu qui suit l'état de la protection, avec ses réglages (actif, vitesse, nombre de points) dans « réglages ».
- Compteur de requêtes bloquées et classement des domaines qui bloquent le plus (page Résolveur DNS).
- Fiche projet `docs/FICHE.md`, `SECURITY.md` et ce journal des versions.
- Outils `tools\` communs aux applis Aria : tests, version, vérification d'une release, protection du dépôt.

- Icône de la zone de notification selon le contrat : pastille orange (à regarder), cyan (mode Discret) ou bleue (nouvelle version publiée) ; menu avec « Aller à », « Voir la mise à jour » et la case « Démarrer avec Windows ».

### Modifié

- Habillage de l'interface selon le contrat des applis Aria : panneaux à coins coupés et liseré cyan (accentué sur le panneau du bouton orange), boutons en biais, sous-onglets en encoche avec leur encadré « À savoir » (Résolveur DNS, VPN, Tests, Réglages), cartes à onglet pour les choix exclusifs (mode, liste de blocage, vitesse du fond).

### Corrigé

- L'icône des notifications n'est plus déposée dans `%TEMP%` : Aria fait de `.Aria-Shield\cache` son dossier temporaire.

### Sécurité

- Dépendances mises à jour : `golang.org/x/sys` 0.50 et `vite` 8.3.8.
- La compilation sépare la job qui lit (tests, exe) de celle qui peut écrire (release) ; un tag doit être sur la branche principale.

## 0.11.0 (2026-11-11)

### Ajouté

- Icône de la zone de notification qui suit l'état de la protection, avec un menu pour agir sans ouvrir la fenêtre.
- Suspension du blocage des pubs pendant dix minutes, et choix de la liste (OISD small ou big).
- Avertissement quand une autre version d'Aria est lancée alors qu'une tourne déjà, et signalement d'une nouvelle version publiée.
- Journal `aria.log`.

### Corrigé

- Accueil : en fenêtre étroite, le détail de chaque moteur passe sous son nom.

## 0.10.0 (2026-11-11)

### Ajouté

- Blocage des pubs et des traceurs par le résolveur, en option, avec exceptions.

## 0.9.0 (2026-11-11)

### Ajouté

- Fermer la fenêtre range Aria dans la zone de notification ; démarrage avec Windows ; réactivation de la dernière protection ; notifications Windows.
- Plusieurs profils VPN (jusqu'à 20), import des `.conf`, bascule à chaud.

### Sécurité

- Tor Browser est installé et mis à jour sans droits administrateur ; les dossiers système sont demandés à Windows.

Les versions antérieures sont décrites sur la page des releases.
