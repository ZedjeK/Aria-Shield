# Aria Shield

Un logiciel Windows portable pour naviguer discrètement, en un clic : résolveur DNS à toi, VPN avec kill switch, et Tor Browser.
Rien à installer : tout tient dans un seul exe.

> Version 0.6.1 · Windows 10/11 x64 · licence MIT

## Ce qu'il fait

| Mode | Ce qui est protégé | Comment |
| --- | --- | --- |
| **Résolveur DNS** | Tes requêtes DNS : plus de DNS du FAI, plus de blocage DNS, DNSSEC validé | Unbound tourne depuis Aria en récursion complète, sans aucun DNS tiers |
| **Discret** | Tout le trafic du PC : ton IP est cachée aux sites, ton FAI ne voit plus rien | Tunnel WireGuard vers ton fournisseur VPN + kill switch qui bloque tout ce qui sort hors du tunnel |
| **Fantôme** | La navigation dans Tor Browser : anonymat fort | Tor Browser intégré, vérifié et tenu à jour par Aria. Combiné au mode Discret, ton FAI ne voit même pas que tu utilises Tor |

Une page **Tests** vérifie ce qu'Aria annonce : Windows passe bien par le résolveur, aucune fuite d'IP en IPv4 ou IPv6, kill switch actif, et navigateurs dont le « DNS sécurisé » contournerait Aria.

## Installation

1. Télécharge `Aria-Shield.exe` dans les [Releases](../../releases).
2. Lance-le. Windows SmartScreen prévient que l'exe n'est pas signé : « Informations complémentaires », puis « Exécuter quand même ».
3. Accepte la demande de droits administrateur : Aria en a besoin pour modifier le DNS, créer la carte VPN et poser le pare-feu.

Aria crée un dossier `AriaShield-data` à côté de l'exe. Pour tout désinstaller, supprime l'exe et ce dossier.

Pour le mode Discret, il te faut un fournisseur VPN qui fournit des configurations WireGuard (Proton VPN, gratuit compris, Mullvad, IVPN…). Tu recopies les valeurs de son fichier `.conf` dans la page VPN.

## Sécurité

- **Rien ne reste derrière toi** : à la désactivation, à la fermeture d'Aria et même en cas de plantage, ton DNS d'origine revient, le kill switch disparaît (session de pare-feu dynamique) et le pilote VPN est retiré.
- **Profil VPN chiffré** avec DPAPI, lié à ton compte Windows. La clé privée n'est jamais réaffichée.
- **Composants vérifiés** : chaque binaire embarqué vient de la source officielle, avec sa signature vérifiée (voir plus bas). Aria revérifie leur empreinte ou leur signature avant chaque utilisation.
- **Tor Browser jamais en administrateur** : Aria tourne en admin, mais lance Tor Browser avec tes droits normaux.

### Limites à connaître

- L'exe n'est pas signé : vérifie l'empreinte SHA-256 publiée dans chaque Release.
- Le mode Fantôme rend anonyme ce que tu fais **dans Tor Browser**, pas le reste du PC.
- Un VPN cache ton IP, il ne te rend pas anonyme vis-à-vis de ton fournisseur VPN.
- Projet personnel, sans audit de sécurité indépendant.

## Provenance des composants embarqués

| Composant | Source | Vérification |
| --- | --- | --- |
| Unbound 1.26.1 | `unbound-1.26.1.zip`, NLnet Labs | Signature PGP « NLnet Labs releases signing key G2 » (`2310 1869 0C4D 903E F419 146A A144 323D EAAC DF45`). SHA-256 de `unbound.exe` : `8fd7f2d2ad66261ea09e345dd0ffcde7428c404f474fcf459ef6c3d05470fa24` |
| WireGuardNT 1.1 | `wireguard-nt-1.1.zip`, download.wireguard.com | Signature Authenticode « WireGuard LLC ». SHA-256 de `wireguard.dll` : `b1b85e072c45d81358be29d94c599dc76652f912be8c0f0a41e2d5d89a6461d3` |
| Tor Browser 15.0.24 | `tor-browser-windows-x86_64-portable-15.0.24.exe`, dist.torproject.org | Signature PGP « Tor Browser Developers (signing key) » (`EF6E 286D DA85 EA2A 4BA7 DE68 4E2C 6E87 9329 8290`), revérifiée par Aria à chaque installation ou mise à jour |

Le kill switch (`src/internal/killswitch`) est dérivé du pare-feu de [wireguard-windows](https://git.zx2c4.com/wireguard-windows/) (MIT) : la seule modification autorise le processus Aria au lieu du service WireGuard.

## Compiler soi-même

Prérequis : [Go](https://go.dev/dl/) 1.25 ou plus et [Node.js](https://nodejs.org/) LTS.

```powershell
powershell -ExecutionPolicy Bypass -File .\build.ps1
```

Le script télécharge Tor Browser 15.0.24 sur dist.torproject.org (trop lourd pour le dépôt), installe le CLI [Wails v3](https://v3.wails.io/) si besoin, génère le projet dans `.work\`, y applique `src\` et produit `Aria-Shield.exe` à la racine.

## Structure

```text
src/
├── main.go                     # appli Wails, fenêtre
├── shieldservice.go            # méthodes appelées par l'interface
├── internal/
│   ├── dnsengine/              # Unbound embarqué : extraction, lancement, tests
│   ├── sysdns/                 # bascule et restauration du DNS de Windows
│   ├── vpn/                    # profil chiffré, tunnel WireGuardNT
│   ├── killswitch/             # pare-feu WFP (dérivé de wireguard-windows, MIT)
│   ├── leak/                   # mesure de l'IP publique
│   ├── torbrowser/             # Tor Browser embarqué, mises à jour signées
│   ├── browsers/               # détection du DNS sécurisé des navigateurs
│   └── sysx/                   # commandes sans console, droits admin
├── frontend/src/               # interface Svelte (polices embarquées)
└── build/windows/              # manifest : requireAdministrator
```

## Licence

Code d'Aria Shield sous licence [MIT](LICENSE). Les composants tiers gardent leur propre licence : voir [LICENCES-TIERS.txt](LICENCES-TIERS.txt).
