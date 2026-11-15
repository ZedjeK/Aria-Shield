# Journal des versions

Un bloc par version, le plus récent en haut. Les notes complètes d'une version
sont dans `docs/releases/`.

## Non publié

### Ajouté

- Compteur de requêtes bloquées et classement des domaines qui bloquent le plus (page Résolveur DNS).
- Fiche projet `docs/FICHE.md`, `SECURITY.md` et ce journal des versions.
- Outils `tools\` communs aux applis Aria : tests, version, vérification d'une release, protection du dépôt.

### Corrigé

- L'icône des notifications n'est plus déposée dans `%TEMP%`.

### Sécurité

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
