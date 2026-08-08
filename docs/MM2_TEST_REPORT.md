# Rapport de validation du Market Module 2.0 No-Mint

> Statut : campagne locale terminée
> Date de démarrage : 16 juillet 2026
> Dernière mise à jour : 8 août 2026
> Langue : français
> Conclusion finale : **NO-GO pour test communautaire public et mainnet en l'état**

## Partie I — Tests appliqués et corrections

Cette partie rassemble les scénarios exécutés, leurs preuves et les corrections apportées aux anomalies révélées par les tests. La partie II est réservée aux mécanismes ajoutés pour améliorer la conformité, la sécurité ou la maintenabilité de MM2 au-delà de la correction directe d'un test en échec.

### 1. Résumé exécutif

Ce rapport documente la validation locale de l'implémentation No-Mint du Market Module 2.0 pour Terra Classic. L'objectif est de vérifier la conformité fonctionnelle, la sûreté économique et la résistance aux pannes avant une éventuelle ouverture à des tests communautaires.

La campagne distingue strictement :

- les échecs du code MM2 ;
- les limites ou erreurs de l'environnement de test ;
- les écarts entre la proposition et l'implémentation ;
- les risques qui ne peuvent être évalués complètement qu'en testnet public.

Les suites Go existantes passent intégralement et les invariants économiques No-Mint ajoutés sont validés. Sur le devnet mono-validateur, les swaps puisent bien dans le pool préfinancé, les frais sont répartis conformément au profil local, les plafonds sont atomiques et les rotations d'époque détruisent les soldes résiduels sans mint Market.

Cette réussite fonctionnelle ne permet toutefois pas encore d'ouvrir le module à la communauté. Les trois défauts critiques identifiés ont été corrigés et revalidés localement : contrat du taux `UST`, initialisation de `market_accumulator`, puis migration v15 avec déploiement inactif et première activation différée. Ces corrections ne sont pas encore intégrées à des versions publiées et l'upgrade doit encore être répété sur un snapshot pré-v15 représentatif.

L'audit de conformité avait également identifié l'absence d'adaptation de `base_pool` et `pool_recovery_period`. Ce mécanisme est maintenant implémenté et revalidé localement, avec le plafond direct par actif conservé comme protection stricte. L'arrêt persistant après 25 blocs sous 50 % de puissance Oracle, son réarmement et le véritable TWAP sur 45 blocs sont également implémentés et couverts par des tests déterministes.

GAP-004 a finalement été requalifié : le frein de gouvernance existait déjà sous la forme de `MinStabilitySpread = 100 %`, valeur historiquement utilisée sur Columbus-5. Il ferme les swaps en produisant une sortie nette nulle. La route accélérée possède aussi le seuil `0,667`, mais son dépôt minimal conservait le denom générique `stake`, inutilisable sur Terra Classic. Le genesis personnalisé et la migration v15 normalisent désormais les deux dépôts en `uluna`. La fermeture, l'absence de mutation, la persistance et la réouverture sont testées.

Le blocage `E2E-001` est également levé. Le harnais officiel démarre désormais quatre validateurs natifs ARM64, confirme leurs connexions P2P et produit des blocs. Le scénario ciblé construit un historique TWAP par trois cycles Oracle complets, puis exécute avec succès les swaps LUNC → USTC et USTC → LUNC. Le verdict reste néanmoins **NO-GO** jusqu'à publication des correctifs locaux, essai d'upgrade sur snapshot et extension des scénarios multi-validateur aux pertes de quorum et à la gouvernance.

### 2. Références figées

| Élément | Référence testée |
|---|---|
| Proposition MM2.0 No-Mint | `Market-Module-2-0/proposal@e576826a8163d4aacb64be0709822399dd970d5f` |
| Code Terra Classic MM2 | `8afe857005021519f314eae091c12ea7f3865fb1`, puis correctifs locaux INT-002, INT-003 et GAP-001 à GAP-004 |
| Branche locale | `mm2-development` |
| Branche source | `upstream/feat/mm-implementation` |
| Feeder Oracle StrathCole | `89cd2983015f47aa6fe005ebc3eb35e24789ba1a`, puis correctif local `mm2-ust-price` |
| Image du nœud | `terra-classic-devnet:mm2` (`441d70cc604d`) |
| Image du feeder | baseline `ee5143babfc8`, correctif local `952be0ca1363` |
| Image E2E multi-validateur | `terra:debug`, construite nativement pour `linux/arm64` |

### 3. Environnement

| Composant | Valeur |
|---|---|
| Machine | Apple Silicon ARM64 |
| Système hôte | macOS / Darwin 25.5.0 |
| Go | 1.24.7 darwin/arm64 |
| Docker Engine | 28.1.1 linux/arm64 |
| Docker Desktop | 4.41.2 |
| Chain ID | `mm2-local-1` |
| Nœud | sain |
| Feeder Oracle | sain |
| Vote Oracle | toutes les 5 hauteurs |
| Époque MM2 locale | 100 blocs |

Le nœud, ses données et ses ports sont isolés du `terrad` éventuellement installé sur l'hôte. Les secrets du validateur ne sont pas inclus dans ce rapport.

### 4. Paramètres MM2 observés

| Paramètre | Valeur locale |
|---|---:|
| Spread minimum | 0,35 % |
| Burn des frais de swap | 50 % |
| Community Pool | 0 % |
| Reste des frais vers Oracle | 50 % |
| Ancienneté Oracle maximale | 75 secondes |
| Fenêtre TWAP | 45 blocs |
| Déviation TWAP maximale | 10 % |
| Plafond journalier strict | 10 % de la baseline |
| Facteur du calcul adaptatif | 7 % |
| Redirection fiscale vers l'accumulateur | 60 % |

### 5. Résultats synthétiques

| ID | Domaine | Test | Statut |
|---|---|---|---|
| BASE-001 | Compilation/tests | Modules Market, Oracle, Tax et Treasury sans cache | PASS |
| BASE-002 | Régression | Suite complète `go test -count=1 ./...` | PASS |
| UNIT-001 | Invariants | No-Mint et comptabilité des frais dans les deux sens | PASS |
| UNIT-002 | Époque | Burn des soldes puis refill sans création monétaire | PASS |
| FUZZ-001 | Robustesse | Fuzzing des cotations positives, 118 368 exécutions | PASS |
| ENV-001 | Environnement | Exécution Go dans le sandbox avec cache global | ENVIRONMENT |
| ORA-001 | Oracle live | Feeder sain et connecté au nœud | PASS |
| ORA-002 | Oracle live | Prévotes et votes exécutés on-chain | PASS |
| ORA-003 | Oracle live | Taux `uusd` et `UST` disponibles on-chain | PASS |
| ORA-004 | Panne Oracle | Refus atomique, suppression des taux et reprise du feeder | PASS ; auto-kill GAP-002 couvert localement |
| ENV-002 | Profil local | Taux `usdr` requis par le pool virtuel | RESOLVED |
| INT-001 | Core ↔ feeder | Sémantique du méta-denom `UST` | **RESOLVED LOCAL — PR EN ATTENTE** |
| INT-002 | Tax ↔ Market | Initialisation de `market_accumulator` | **RESOLVED LOCAL — PR EN ATTENTE** |
| INT-003 | Upgrade v15 | Migration et première activation d'une chaîne pré-MM2 | **RESOLVED LOCAL — PR EN ATTENTE** |
| TAX-001 | Fiscalité E2E | Redirection de 60 % vers l'accumulateur | PASS |
| EPOCH-001 | Époque E2E | Burn intégral du pool précédent et refill intégral | PASS |
| SWAP-001 | Swap E2E | LUNC → USTC, transfert depuis le pool et frais 50/50 | PASS après correction INT-001 |
| SWAP-002 | Swap E2E | USTC → LUNC, transfert depuis le pool et frais 50/50 | PASS après correction INT-001 |
| SAFE-001 | Cap journalier | Refus au-delà de 10 % et atomicité | PASS |
| SAFE-002 | Paire autorisée | Refus USTC → SDR et atomicité | PASS |
| LOAD-001 | Charge | 20 swaps séquencés et comportement à la frontière d'époque | PASS |
| RES-001 | Redémarrage | Persistance des hauteurs, soldes et taux Oracle | PASS |
| RES-002 | Export/import | Export complet et reprise dans un home vierge | PASS — export par défaut corrigé localement |
| E2E-001 | Multi-validateur | Harnais officiel à quatre validateurs, Oracle, TWAP et swaps bidirectionnels | **RESOLVED — TEST CIBLÉ PASS** |
| IMP-001 à IMP-007 | Améliorations | Liquidité adaptative, durcissement de l'époque, registre d'actifs, auto-kill Oracle, véritable TWAP et frein de gouvernance | **IMPLEMENTED LOCAL — TESTS PASS** |
| GAP-002 | Sécurité Oracle | Auto-kill quorum utilisant l'état persistant d'activation | **RESOLVED LOCAL — PANNE MULTI-VALIDATEUR À TESTER** |
| GAP-003 | TWAP | Moyenne réellement pondérée et phase d'amorçage sûre | **RESOLVED LOCAL — TESTS PASS** |
| GAP-004 | Gouvernance | Fermeture et réouverture par le spread historique, route accélérée et persistance | **RESOLVED LOCAL — TESTS PASS** |
| CORE-001 | Exploitation | `terrad export` sans liste de modules | **RESOLVED LOCAL — TESTS PASS** |

`ENV-001` et `ENV-002` ne sont pas des échecs fonctionnels MM2. `E2E-001` était également un défaut d'infrastructure et est maintenant résolu. `CORE-001`, désormais corrigé localement, appartient au fonctionnement général du dépôt et non à la logique économique MM2.

### 6. Preuves de baseline

#### BASE-001 — Modules directement concernés

Commande :

```bash
go test -count=1 ./x/market/... ./x/oracle/... ./x/tax/... ./x/treasury/...
```

Résultat : tous les packages contenant des tests ont retourné `ok`.

#### BASE-002 — Dépôt complet

Commande :

```bash
go test -count=1 ./...
```

Résultat : tous les packages contenant des tests ont retourné `ok`. Aucun panic, échec de compilation ou test en échec n'a été observé.

#### UNIT-001, UNIT-002 et FUZZ-001 — Invariants ajoutés

Des tests déterministes couvrent les deux directions de swap, l'absence de mint, la conservation des flux entre trader et pool, le burn, le compte Oracle, ainsi que le burn/refill d'époque. Ils passent sans cache. Un fuzzing de 30 secondes sur le calcul de cotation a exécuté 118 368 cas sans panic, valeur négative ou résultat nul inattendu.

#### TAX-001 — Répartition fiscale on-chain

Une transaction taxable a produit `1 000 000 uluna` et `500 000 uusd` de taxe. Les événements et les soldes confirment la répartition suivante :

| Destination | `uluna` | `uusd` | Part |
|---|---:|---:|---:|
| Community Pool | 800 | 400 | 0,08 % |
| Oracle | 39 200 | 19 600 | 3,92 % |
| `market_accumulator` | 600 000 | 300 000 | **60 %** |
| Burn | 360 000 | 180 000 | 36 % |

Transaction : `432EAA6B05C98E2AD94616F17BB163768B5FE9E40BF2703476B48239C0AA10E9`, hauteur 1558.

#### EPOCH-001 — Rotation du pool

À la frontière d'époque 1600/1601, l'intégralité du solde de l'accumulateur a été déplacée vers `market` : `9 999 600 000 uluna` et `99 800 000 uusd`. À la frontière suivante 1700/1701, après les deux swaps, le pool résiduel de `9 818 973 524 uluna` et `100 794 495 uusd` a été intégralement brûlé. La supply `uusd` a diminué exactement de `100 794 495` dans ce bloc. La supply `uluna` reflète simultanément le burn du pool et l'émission normale du module Mint ; les événements permettent de séparer ces deux flux.

#### SWAP-001 et SWAP-002 — Exécution réelle et comptabilité

| Sens | Offre | Reçu | Frais | Burn | Oracle | Hash |
|---|---:|---:|---:|---:|---:|---|
| LUNC → USTC | `1 000 000 uluna` | `5 486 uusd` | `19 uusd` | `9 uusd` | `10 uusd` | `DC0CE1B0…F2201D` |
| USTC → LUNC | `1 000 000 uusd` | `180 990 784 uluna` | `635 692 uluna` | `317 846 uluna` | `317 846 uluna` | `D6B7DAF1…ED657B` |

Dans les deux cas, l'offre a été créditée au compte `market`, la sortie et les frais ont été débités de ce même compte, et aucun événement de mint n'a été émis par Market. Les montants sont mécaniquement cohérents avec le taux Oracle reçu, mais économiquement incorrects à cause de `INT-001`.

#### SAFE-001 et SAFE-002 — Refus atomiques

Un swap de `2 000 000 000 uluna`, coté à `10 973 993 uusd`, dépassait le cap de 10 % d'un pool de référence à `99 800 000 uusd`. La transaction `42631C30…1AB3E6` a été refusée avec le code Market 9 (`daily swap cap exceeded`). Les soldes du pool sont restés exactement `9 999 600 000 uluna` et `99 800 000 uusd`.

Une tentative `1 000 000 uusd` vers `usdr` (`AA414446…D1C19`) a été refusée avec le code Market 5 (`invalid swap pair; not allowed`). Le pool est également resté inchangé. Dans les deux cas, seuls les frais de transaction Ante ont été facturés au signataire, conformément au fonctionnement Cosmos SDK ; aucun état produit par le message n'a été conservé.

#### ORA-004 — Panne et reprise du feeder

Le feeder a été arrêté pendant plus de 75 secondes. L'Oracle natif a retiré les taux qui n'atteignaient plus son seuil de vote. Le swap `F8E352D9…` à la hauteur 1971 a été refusé avec le code Market 3 (`no price registered with oracle`) et le pool est resté inchangé. Le code spécifique `oracle price stale` n'a pas été atteint, car la suppression du taux intervient avant la limite de fraîcheur MM2 dans ce profil.

Après redémarrage, le feeder est redevenu sain et les taux `UST`, `uusd` et `usdr` ont été revotés en environ 20 secondes. Cette expérience, réalisée avant le correctif de GAP-002, valide le refus sûr et la récupération opérationnelle mais ne constitue pas une preuve E2E de l'auto-kill. Le mécanisme de 25 blocs est désormais couvert par les tests déterministes décrits sous `IMP-005` ; sa répétition sur un réseau multi-validateur reste requise.

#### LOAD-001 — Série de swaps et frontière d'époque

Vingt transactions de swap ont été envoyées avec des séquences de compte valides. Les quinze premières ont été incluses avec le code 0. La frontière d'époque à la hauteur 4801 a ensuite brûlé le pool résiduel ; les cinq transactions suivantes ont été refusées avec le code Market 6 (`insufficient pool liquidity`). Aucun débit partiel du pool n'a été observé.

Un envoi réellement simultané depuis une seule adresse a d'abord exposé la gestion de nonce du client, pas un défaut MM2. La campagne ne revendique donc pas un benchmark de débit parallèle ; elle valide la répétition des swaps, l'atomicité et le changement d'état concurrent avec une frontière d'époque.

#### RES-001 — Redémarrages

Le nœud a été redémarré à partir de la hauteur 2040 et est redevenu sain à la hauteur 2056. Les soldes, les paramètres et l'état du Market ont persisté ; le feeder s'est reconnecté et a repris ses votes. Plusieurs arrêts contrôlés ultérieurs, autour des hauteurs 13 858 à 14 063, ont confirmé le même comportement.

#### RES-002 et CORE-001 — Export/import

L'export ciblé des modules `auth`, `bank`, `market`, `oracle`, `treasury` et `tax` a produit un JSON valide contenant tous les paramètres MM2 attendus. Un export complet a ensuite réussi en donnant explicitement les 23 modules réellement enregistrés. Le fichier résultant avait une hauteur initiale 14 063 et un validateur.

Ce genesis a été monté dans un conteneur neuf, sans le volume du devnet. Avec la clé du validateur exclusivement locale copiée temporairement, la chaîne importée a finalisé les blocs 14 063 à 14 066. La copie de clé a ensuite été supprimée.

**Constat initial.** La commande par défaut suivante échouait :

```bash
terrad export --home /var/lib/terra --height -1
```

Erreur : `module crisis does not exist`. Le nom `crisis` figurait dans les ordres de début de bloc, de fin de bloc et d'initialisation/export, alors que ce module n'est pas enregistré dans `appModules`. Cette incohérence générale du core n'altérait pas les données MM2, mais imposait le contournement `--modules-to-export`.

**Solution implémentée.** Les trois références obsolètes à `crisis` ont été retirées de ces ordres. Le module n'a pas été réintroduit, car il ne fait pas partie de l'application actuellement assemblée. Un test de régression générique construit l'application en mémoire et vérifie désormais que chaque nom présent dans les ordres de début de bloc, de fin de bloc, d'initialisation et d'export correspond à un module réellement enregistré.

**Résultat.** L'image Docker corrigée a été reconstruite, puis `terrad export` a été exécuté sans `--modules-to-export` sur un nœud réel. La commande a terminé avec le code 0 et produit les 23 modules enregistrés, dont `market`; aucun état `crisis` absent n'a été annoncé. Le test de cohérence des ordres et la suite du package `app` passent également. `CORE-001` est donc résolu localement et le contournement n'est plus nécessaire.

#### E2E-001 — Harnais officiel multi-validateur

**Statut initial : BLOCKED — ENVIRONNEMENT**
**Statut actualisé : RESOLVED — TEST CIBLÉ PASS**

##### Constat initial

Le test officiel ciblé `TestIntegrationTestSuite/TestMarketSwap` créait quatre conteneurs, mais la chaîne restait à la hauteur 0. Les causes se cumulaient : chaque configuration P2P contenait le propre identifiant du nœud, les validateurs étaient démarrés séquentiellement alors que le premier attendait déjà le consensus, et l'image imposait `linux/amd64` sur un hôte Apple Silicon. Sous émulation, les connexions P2P échouaient notamment avec :

```text
secret conn failed: failed to decrypt SecretConnection:
chacha20poly1305: message authentication failed
```

Une fois le consensus rétabli, d'autres défauts du scénario sont devenus visibles : gas Oracle supérieur à la limite Ante, délégation feeder redondante, parseur incompatible avec le JSON legacy des comptes modules, sel Oracle trop long, utilitaire de vote capable de prendre une erreur CLI pour un succès, fenêtre de fraîcheur de deux secondes incompatible avec quatre validateurs et tentative de swap avant constitution complète du TWAP.

##### Solution implémentée

- chaque nœud reçoit tous les pairs persistants sauf lui-même ;
- les quatre conteneurs sont démarrés avant toute attente de consensus, puis chacun doit voir les trois autres pairs et avancer de plusieurs blocs ;
- l'image E2E suit l'architecture cible Docker et sélectionne la bibliothèque `wasmvm` correspondante ;
- les transactions Oracle utilisent explicitement la limite de `1 000 000` gas et leurs erreurs de diffusion ou d'exécution sont bloquantes ;
- le test utilise le feeder implicite du validateur, des sels de quatre caractères et un parseur compatible avec les formats JSON legacy et protobuf ;
- la fraîcheur E2E reprend la valeur de production de 75 secondes ;
- le scénario exécute trois cycles prevote/vote complets afin que la première observation couvre réellement la fenêtre TWAP de 45 blocs avant tout swap ;
- les réserves actives sont alimentées juste avant les transactions, après les rotations d'époque susceptibles de survenir pendant l'amorçage.

Des tests de régression vérifient l'exclusion du pair propre, la présence des autres pairs, le démarrage complet du groupe avant les attentes et les deux formats de comptes modules.

##### Résultat

Le 8 août 2026, le test ciblé a réussi en `138,61 s` sur quatre validateurs `linux/arm64`. Chaque validateur a soumis trois prevotes et trois votes Oracle valides. Les taux ont été tallyés, le TWAP complet a été accepté, puis les transactions suivantes ont été incluses avec le code 0 :

- `1 000 000 uluna` vers `uusd`, avec baisse du solde LUNC et hausse du solde USTC du trader ;
- `500 000 uusd` vers `uluna`, avec baisse du solde USTC et hausse du solde LUNC du trader.

`E2E-001` est donc fermé comme défaut d'infrastructure. Cette preuve couvre le chemin consensus → Oracle → TWAP → Market sur quatre validateurs de même puissance. Elle ne couvre pas encore une panne volontaire de quorum, des puissances inégales, une fermeture de gouvernance ou l'intégralité de la suite E2E avec IBC et state-sync.

### 7. Matrice de validation

| Domaine | Exigence principale | Niveau actuel |
|---|---|---|
| Swap | LUNC ↔ USTC seulement | PASS, stable→stable refusé |
| No-Mint | aucune hausse de supply Market lors d'un swap | PASS unitaire et E2E |
| Frais | 0,35 %, 50 % burn, 50 % Oracle | PASS avec paramètres locaux forcés |
| Liquidité | aucune sortie supérieure au pool | PASS |
| Atomicité | aucun état du message après un refus | PASS |
| Fiscalité | 60 % vers l'accumulateur | PASS ; migration locale couverte |
| Époque | burn des restes puis refill | PASS avec époque locale de 100 blocs |
| Oracle | prix USTC réel et récent | PASS après correction locale du feeder |
| TWAP | refus au-delà de 10 % d'un TWAP réellement pondéré sur 45 blocs | PASS déterministe ; historique incomplet refusé |
| Cap journalier | maximum 10 % et reset | PASS E2E pour le dépassement |
| Quorum | arrêt sous 50 % de puissance pendant 25 blocs | PASS déterministe pondéré ; chemin Oracle sain PASS sur quatre validateurs, panne E2E restant à jouer |
| Gouvernance | fermeture accélérée à 0,667 et activation différée | PASS local ; mécanisme historique réutilisé, E2E multi-validateur restant |
| Liquidité adaptative | recalcul de `base_pool` et PRP à l'époque | PASS local ; facteur adaptatif de 7 % |
| Résilience | restart et export/import | PASS, export par défaut inclus |
| Upgrade | migration d'un état pré-MM2 | PASS local sur état sans les nouvelles clés ; snapshot réel restant |

### 8. Anomalies confirmées et corrections

#### INT-001 — Incompatibilité de format du taux `UST`

**Statut : FAIL**
**Gravité : critique / P0**
**Périmètre : intégration entre le core MM2 et le feeder Oracle StrathCole**

Le core MM2 documente et utilise le méta-denom `UST` comme le prix USD d'un USTC. Le feeder testé convertit au contraire `USTC/USD` en un taux `USTC par LUNC` avant de le soumettre. Les deux composants sont donc fonctionnels isolément, mais leur contrat de données n'est pas compatible.

Valeurs observées pendant le test :

| Donnée | Valeur |
|---|---:|
| Prix LUNC/USD exposé par le feeder | `0.000059107250373` |
| Prix USTC/USD exposé par le feeder | `0.0054963836173671` |
| Taux `UST` voté on-chain | `0.010754254719151968` |
| Cotation de 1 LUNC vers USTC après spread | `0.005477 USTC` |
| Cotation de 1 USTC vers LUNC après spread | `181.287156 LUNC` |

Le taux on-chain `UST` correspond approximativement à :

```text
LUNC/USD ÷ USTC/USD = USTC par LUNC
```

Le core le traite ensuite comme s'il représentait :

```text
USD par USTC
```

Conséquence : les cotations MM2 ne correspondent pas au ratio de marché LUNC/USTC. Dans l'échantillon ci-dessus, la cotation LUNC vers USTC est proche de la moitié de la valeur attendue, tandis que la cotation inverse est presque doublée.

Commandes de reproduction :

```bash
terrad query oracle exchange-rates --output json
terrad query market swap 1000000uluna uusd --output json
terrad query market swap 1000000uusd uluna --output json
```

**Impact :** aucun testnet communautaire ne devrait être ouvert avec cette combinaison core/feeder, car les swaps seraient économiquement mal valorisés même si les votes Oracle et les transactions sont techniquement valides.

##### Re-test après correction locale du feeder

**Statut actualisé : RESOLVED LOCAL — PR EN ATTENTE**

Le contrat retenu est celui déjà documenté par le core : `UST` transporte directement le prix `USD par USTC`. La branche locale `mm2-ust-price` du feeder traite désormais ce méta-denom comme une exception et ne lui applique plus la conversion historique `fiat par LUNC`.

Valeurs observées le 5 août 2026 après reconstruction de l'image Oracle :

| Donnée | Valeur |
|---|---:|
| Prix LUNC/USD exposé par le feeder | `0.0000501052732435` |
| Prix USTC/USD exposé par le feeder | `0.0049910125303164` |
| Taux `UST` voté on-chain | `0.004991012530316400` |
| Cotation de 1 LUNC vers USTC après spread | `0.010003 USTC` |
| Cotation de 1 USTC vers LUNC après spread | `99.261887 LUNC` |

Deux transactions réelles ont ensuite confirmé les deux directions et la comptabilité des frais :

| Sens | Offre | Reçu | Frais MM2 | Burn | Oracle | Hauteur | Hash |
|---|---:|---:|---:|---:|---:|---:|---|
| LUNC → USTC | `1 000 000 uluna` | `9 981 uusd` | `35 uusd` | `17 uusd` | `18 uusd` | 394787 | `01652C80…59D5556F` |
| USTC → LUNC | `1 000 000 uusd` | `99 481 916 uluna` | `349 409 uluna` | `174 704 uluna` | `174 705 uluna` | 394797 | `B7A66F4D…DF05A78` |

Les tests déterministes du core et les tests du composant voter passent avec la même définition. `INT-001` est donc isolé et corrigé localement. Il ne sera considéré comme fermé pour une version communautaire qu'après commit, revue et intégration du correctif dans une version publiée du feeder.

#### ENV-002 — Taux `usdr` absent du premier profil local

**Statut : RESOLVED**
**Classification : environnement de test, pas défaut du core**

La première whitelist locale ne contenait que `uusd` et `UST`. Le calcul constant-product du Market Module utilise encore `usdr` comme unité interne et refusait donc toute cotation avec `no price registered with oracle`.

Le profil de test a été corrigé en ajoutant `usdr` à la whitelist et la source officielle IMF au feeder. Le taux a ensuite été voté on-chain et les requêtes de cotation ont pu atteindre le calcul MM2.

#### INT-002 — Le compte module `market_accumulator` n'est pas initialisé au genesis

**Statut : FAIL**
**Gravité : critique / P0**
**Périmètre : intégration entre les modules Tax, Bank, Auth et Market**

Sur une chaîne fraîche, le genesis du module Market initialise explicitement le compte module `market`, mais pas `market_accumulator`. Il est alors possible qu'un envoi bancaire adressé à l'adresse déterministe de l'accumulateur crée d'abord un compte utilisateur ordinaire (`BaseAccount`) à cette adresse. Le post-handler fiscal essaie ensuite d'y transférer la part de taxe au moyen d'un transfert module-vers-module. Le SDK détecte que l'adresse existante n'est pas un `ModuleAccount`, déclenche un panic, puis BaseApp récupère ce panic et fait échouer la transaction.

Transaction de reproduction locale :

```text
C297FA5A49B5E9D51C1D0A144ECCE5327467EB360A7185FA62FA0D730ECF092C
```

Résultat observé à la hauteur 310 :

```text
code: 111222
raw_log: account is not a module account
```

La stack trace situe l'échec dans `tax/keeper.ProcessTaxSplits`, au moment du transfert de `fee_collector` vers `market_accumulator`. Aucun montant envoyé n'a été crédité à la destination. Le nœud est resté sain : il s'agit d'un panic transactionnel récupéré par BaseApp, pas d'un arrêt du processus.

Cause confirmée dans le code :

- `x/market/genesis.go` force seulement la création du compte module `market` ;
- le SDK crée paresseusement un compte module absent, mais panique si un compte ordinaire occupe déjà son adresse déterministe ;
- `market_accumulator` est déclaré comme destinataire autorisé, son existence devrait donc être garantie avant toute transaction utilisateur.

**Impact :** une transaction envoyée à l'adresse de l'accumulateur avant son initialisation peut échouer et empêcher la mise en place attendue de ce compte module. Le chemin fiscal MM2 et l'alimentation du pool ne doivent pas dépendre de l'ordre des premières transactions de la chaîne.

##### Re-test après correction locale du core

**Statut actualisé : RESOLVED LOCAL — PR EN ATTENTE**

Le core garantit désormais le compte `market_accumulator` dans deux chemins :

- `InitGenesis` crée explicitement le compte module lorsqu'il est absent ;
- le handler d'upgrade v15 exécute la même opération de manière idempotente avant les migrations.

Si l'adresse déterministe existe déjà sous forme de `BaseAccount`, elle est convertie en `ModuleAccount`. Le numéro de compte et la séquence sont conservés. Les soldes restent inchangés, car ils sont enregistrés par adresse dans le module Bank.

Deux tests de régression couvrent la création depuis un état absent et la conversion d'un compte bancaire possédant `12 345 uusd`. Le second appelle deux fois l'initialisation pour vérifier l'idempotence. Après conversion, le nom du compte module, son numéro, sa séquence et son solde sont tous préservés.

Validation exécutée :

```bash
go test -count=1 ./x/market/...
go test -count=1 ./app/upgrades/v15 ./x/tax/keeper ./x/treasury/...
go test -count=1 ./...
```

Toutes les suites du core passent. `INT-002` sera considéré comme fermé pour une version communautaire après revue et intégration du correctif.

#### INT-003 — Le handler d'upgrade v15 ne migrait pas l'état pré-MM2

**Statut : FAIL**
**Gravité : critique / P0**
**Périmètre : activation sur une chaîne Terra Classic existante**

Lors de l'audit initial, le handler `app/upgrades/v15/upgrades.go` limitait en mémoire les swaps à `uusd`, ajoutait le méta-denom Oracle `UST` si nécessaire, puis lançait les migrations déclarées par les modules. Même avec la garantie du compte `market_accumulator` apportée par `INT-002`, il n'écrivait alors aucune des nouvelles clés de paramètres :

- `EpochLengthBlocks` ;
- `SwapFeeBurnRate` et `SwapFeeCommunityRate` ;
- `MaxOracleAgeSeconds` ;
- `TWAPLookbackWindow` et `MaxTWAPDeviation` ;
- `DailyCapFactor` ;
- `TaxRedirectRate` dans Treasury.

Dans `x/params`, `Subspace.Get` panique lorsque la clé n'existe pas. Une chaîne créée avant MM2 ne possède pas ces clés. Or chaque `EndBlock` appelle `ProcessEpochIfDue`, qui lit immédiatement `EpochLengthBlocks`. Le risque n'est donc pas une simple mauvaise valeur par défaut : le premier bloc suivant l'upgrade peut arrêter l'exécution de la chaîne.

Même en supposant les clés présentes, ce handler initial ne remplaçait pas le spread de 100 % utilisé historiquement pour désactiver les swaps sur Columbus-5. Les valeurs par défaut du code audité ne correspondaient pas non plus au vote : spread 2 % au lieu de 0,35 %, burn des frais 0 % au lieu de 50 %, donc reliquat Oracle 100 % au lieu de 50 %. Enfin, aucune activation en deux étapes n'était implémentée.

**Impact :** le chemin d'upgrade mainnet n'est pas exécutable en sécurité et le profil fonctionnel observé sur le devnet n'est obtenu que parce que le genesis local force explicitement les valeurs souhaitées.

##### Re-test après correction locale du core

**Statut actualisé : RESOLVED LOCAL — PR EN ATTENTE**

Le handler v15 réalise désormais une migration idempotente qui :

- garantit le compte module `market_accumulator` ;
- conserve les anciennes valeurs `BasePool` et `PoolRecoveryPeriod` ;
- remplace le spread historique par `0,35 %` ;
- initialise l'époque à 30 jours, la répartition de frais 50 % burn / 50 % Oracle, la fraîcheur Oracle à 75 secondes, la fenêtre TWAP à 45 blocs, sa déviation maximale à 10 % et le plafond journalier à 10 % ;
- initialise la redirection fiscale Treasury à 60 % ;
- remplace le denom générique `stake` des dépôts de gouvernance ordinaires ou accélérés par `uluna`, sans changer leurs montants ni leurs seuils ;
- inscrit un état Market persistant désactivé et ancre la première époque de collecte à la hauteur d'upgrade ;
- conserve l'état et la hauteur d'origine si le handler est rejoué.

Le processeur d'époque active ensuite les swaps uniquement après une époque complète et après transfert réussi de l'accumulateur vers des réserves Market contenant à la fois `uluna` et `uusd`. Avant cette frontière, toute transaction de swap retourne explicitement `market module is disabled`. L'état d'activation, l'attente initiale et la dernière hauteur d'époque sont inclus dans l'export/import du genesis.

Les tests ajoutés couvrent deux niveaux :

1. une application en mémoire dont les sept nouvelles clés Market, la clé Treasury et les clés d'activation sont supprimées pour reproduire un état pré-MM2 ;
2. la séquence hauteur 100 → hauteur 110 avec swap refusé pendant la collecte, alimentation des deux réserves, activation automatique, puis premier swap LUNC → USTC réussi.

Validation exécutée :

```bash
go test -count=1 ./app/upgrades/v15
go test -count=1 ./x/market/...
go test -count=1 ./...
```

La prochaine validation d'exploitation devra encore exécuter le binaire sur une copie d'un snapshot pré-v15 représentatif, produire plusieurs blocs après upgrade et confirmer le flux fiscal réel pendant les 30 jours simulés. Cette étape ne remet pas en cause la fermeture locale du défaut déterministe, mais reste obligatoire avant une version communautaire.

## Partie II — Améliorations réalisées

Cette partie distingue les améliorations déjà implémentées des écarts encore ouverts. Une amélioration reçoit un identifiant `IMP` lorsqu'elle ajoute ou renforce un mécanisme de conception, même si elle provient initialement de l'analyse d'un écart `GAP`.

### 9. Améliorations implémentées

| ID | Amélioration réalisée | Résultat |
|---|---|---|
| IMP-001 | Recalcul adaptatif de `base_pool` et PRP avec un facteur de 7 % | Implémenté et testé |
| IMP-002 | Prévalidation Oracle avant la rotation d'époque afin d'éviter une mutation partielle prévisible | Implémenté et testé |
| IMP-003 | Plafond direct renforcé : frais inclus, baselines purgées et maximum de 10 % imposé | Implémenté et testé |
| IMP-004 | Registre générique reliant denom bancaire, source de prix Oracle, TWAP et paire LUNC autorisée | Implémenté et testé, USTC seul actif de production |
| IMP-005 | Auto-kill Oracle pondéré après 25 blocs sous 50 %, persistance et reprise contrôlée | Implémenté et testé localement ; chemin Oracle sain validé sur quatre nœuds, panne E2E restante |
| IMP-006 | TWAP pondéré par durée sur 45 blocs et fermeture sans historique complet | Implémenté et testé localement |
| IMP-007 | Réutilisation du spread historique comme frein de gouvernance, correction du dépôt accéléré et validation fermeture/réouverture | Implémenté et testé localement |

#### IMP-001 à IMP-003 — Résolution et durcissement de GAP-001

**Statut initial : ABSENT**
**Statut actualisé : RESOLVED LOCAL — 7 % ADAPTATIF / 10 % CAP STRICT**
**Gravité initiale : élevée / P1**

##### Constat initial

La proposition impose un recalcul par époque selon le solde du nouveau pool, la supply LUNC, le facteur de burst et deux plafonds. Le code initial brûlait le pool, transférait l'accumulateur et renouvelait la baseline du cap, mais laissait `BasePool` et `PoolRecoveryPeriod` statiques.

##### Solution implémentée

Le processeur d'époque applique maintenant le modèle suivant, dans les unités micro-SDR historiques du module Market :

```text
PRP = max(14 400, ceil(14 400 × supply LUNC après burn / 1T LUNC))
cap journalier adaptatif souhaité = valeur SDR du nouveau pool LUNC × F, avec F = 0,07
base_pool brut = cap journalier souhaité × PRP / (2 × 14 400)
base_pool = min(base_pool brut, 0,00010 × valeur SDR de la supply, 5 000 000 SDR)
```

Le calcul est effectué avant toute mutation de l'époque. Il utilise le solde LUNC de l'accumulateur, la supply qui subsistera après destruction de l'ancien pool et un taux LUNC/SDR Oracle positif et récent. Si ce taux manque ou est périmé, le burn, le refill et la hauteur d'époque restent inchangés ; le même passage d'époque est retenté au bloc suivant. Après application, le delta du pool virtuel repart à l'équilibre afin qu'une réduction de `base_pool` ne conserve pas un ancien déséquilibre incompatible.

Le plafond journalier direct n'est pas supprimé. Il reste la couche de sûreté stricte par actif physique, tandis que `base_pool` et PRP régulent progressivement la courbe de spread et sa récupération. Le calcul du plafond direct inclut désormais la totalité de la sortie du compte Market — montant reçu par l'utilisateur et frais —, initialise une baseline si un actif a été alimenté hors frontière d'époque et supprime les anciennes baselines et consommations lors de la rotation.

Les tests ajoutés valident :

- le facteur par défaut de 7 % avec 600 millions de LUNC valant 24 000 SDR et une supply de 6,5 billions de LUNC : PRP de 93 600 blocs et `base_pool` de 5 460 SDR ;
- la contraction à 1 billion de LUNC : PRP de 14 400 blocs et `base_pool` de 840 SDR ;
- le plafond proportionnel à la valeur de la supply et le plafond absolu de 5 millions de SDR ;
- le report sans mutation d'une époque privée d'un prix Oracle frais, puis sa réussite après publication du prix ;
- l'application des paramètres à la première activation et l'exécution du premier swap ;
- la comptabilisation des frais dans le plafond direct, la purge des compteurs d'une ancienne époque et le rejet de toute configuration dépassant le plafond absolu de 10 %.

La proposition présente une incohérence rédactionnelle : elle définit explicitement `F = 0,07`, annonce « au plus 10 % », puis utilise `F = 0,1` dans son exemple. L'implémentation locale retient la valeur par défaut explicite de 7 % pour dimensionner `base_pool`. Le taux de 10 % reste uniquement le plafond journalier strict par actif. Un test de régression impose cette séparation afin qu'une modification du plafond strict ne change pas silencieusement le facteur adaptatif. Il reste souhaitable que l'exemple de la proposition soit clarifié avant le mainnet.

#### IMP-004 — Généralisation des actifs Market

##### Constat initial

Le premier code MM2 traitait `uusd` et le méta-denom `UST` par des conditions particulières réparties entre la validation des paires, le calcul du taux, le TWAP, l'activation et l'upgrade. Ajouter un autre actif de cette manière aurait dupliqué les contrôles et multiplié les chemins à corriger.

##### Solution implémentée

Une définition commune `MarketAssetConfig` relie maintenant :

- le denom bancaire réellement détenu par le pool ;
- le denom Oracle qui transporte le prix de marché ;
- le mode de conversion du prix vers l'unité historique « actif par LUNC ».

La validation des paires, la résolution des prix, la collecte TWAP, la condition de liquidité initiale, l'ajout des cibles Oracle lors de l'upgrade et le contrôle de quorum de GAP-002 consomment tous ce même registre déterministe. Aucune condition USTC supplémentaire n'a été introduite pour l'auto-kill.

La configuration de production contient toujours uniquement `uluna ↔ uusd`. Un méta-denom EUTC fictif a été utilisé dans les tests pour démontrer, sans l'activer, que le même code :

- calcule les deux directions à partir de prix USD indépendants ;
- exécute un swap complet depuis une réserve physique sans hausse de supply ;
- collecte les entrées TWAP requises par l'actif configuré ;
- accepte cette réserve pour une première activation ;
- continue de refuser les swaps stable-vers-stable et les actifs absents du registre.

L'ajout réel d'EUTC ou d'un autre actif reste une décision de chaîne : il nécessitera un upgrade coordonné, un denom Oracle définitif, des feeders compatibles, un quorum suffisant, une réserve physique et une campagne de tests dédiée. La généralisation supprime la duplication de code, mais ne contourne pas ces prérequis économiques et opérationnels.

#### IMP-005 — Résolution de GAP-002 : auto-kill Oracle pondéré

**Statut initial : ABSENT**
**Statut actualisé : RESOLVED LOCAL — BASELINE MULTI-VALIDATEUR PASS**
**Gravité initiale : élevée / P1**

##### Constat initial

L'état persistant d'activation introduit par INT-003 permettait de refuser les swaps, mais aucune donnée de puissance ne traversait le hook Oracle et aucun compteur ne mesurait 25 blocs consécutifs sous 50 %. Le retrait natif d'un taux invalide faisait déjà échouer les swaps, sans toutefois implémenter la durée, la persistance de l'arrêt ni la reprise définies par la proposition.

##### Solution implémentée

Le module Oracle transmet désormais au Market, après chaque tally, une observation déterministe pour chaque cible de vote : denom, puissance ayant soumis un prix positif et puissance totale des validateurs actifs. Cette mesure est produite on-chain avant que le tally natif retire les denoms n'atteignant pas son seuil. Elle ne dépend donc ni du feeder officiel, ni du feeder StrathCole, ni du nombre de processus en fonctionnement : seule la puissance de vote des validateurs compte.

Pour la paire de production LUNC/USTC, le registre d'actifs impose simultanément le quorum des deux entrées réellement nécessaires au prix :

- `uusd`, prix USD/LUNC ;
- `UST`, prix USD/USTC.

Après chaque période Oracle, un compteur persistant par denom est incrémenté du nombre de blocs de la période si la puissance est strictement inférieure à 50 %. Une puissance exactement égale à 50 % est suffisante. Une période saine remet immédiatement le compteur du denom à zéro. Dès qu'un compteur atteint 25 blocs, le Market passe dans l'état persistant `oracle_halted` et refuse les swaps. Le compteur est plafonné à 25 afin d'éviter une croissance inutile.

L'arrêt est levé uniquement lorsqu'un même tally montre un quorum suffisant pour toutes les entrées exigées par le registre. Le retour du quorum ne réactive effectivement les swaps que si l'état d'activation de base est lui-même ouvert ; il ne peut donc contourner ni la première activation post-upgrade, ni un futur arrêt de gouvernance. Réciproquement, une frontière d'époque ne peut pas activer le Market tant que `oracle_halted` est vrai. Si cette situation survient à la toute première frontière, l'époque n'est ni consommée ni déplacée : elle est retraitée dès la récupération Oracle, sans imposer 30 jours d'attente supplémentaires. Les transitions d'arrêt et de reprise émettent des événements dédiés avec hauteur, denom fautif et puissances observées.

L'état exporté contient le motif d'arrêt et les compteurs par denom, triés de manière déterministe. L'import restitue donc exactement une panne en cours et sa durée. La migration v15 initialise explicitement le nouvel état sans réinitialiser une installation déjà migrée.

Les tests ajoutés couvrent :

- cinq tallies successifs de 5 blocs à 49 % et l'arrêt uniquement au 25e bloc ;
- la limite exacte de 50 % et la remise à zéro d'un compteur antérieur ;
- un denom complètement absent du résultat Oracle ;
- le maintien de l'arrêt lorsqu'un seul des deux prix récupère son quorum ;
- la reprise lorsque tous les prix requis sont sains ;
- l'interaction dans les deux sens avec la première activation différée, la reprise de sa frontière d'époque et un arrêt indépendant ;
- la persistance export/import ;
- une configuration générique EUTC fictive, sans activation de cet actif en production ;
- le passage réel des puissances pondérées depuis l'EndBlocker Oracle vers le hook Market avec trois validateurs de même puissance.

Cette correction ferme l'écart déterministe dans le code. Le harnais multi-validateur exécute maintenant les votes Oracle sains de quatre validateurs et des swaps bidirectionnels. Il faut encore y reproduire l'arrêt et la reprise avec des puissances inégales avant une proposition mainnet.

#### IMP-006 — Résolution de GAP-003 : véritable TWAP et amorçage fermé

**Statut initial : PARTIEL**
**Statut actualisé : RESOLVED LOCAL — TESTS PASS**
**Gravité initiale : élevée / P1**

##### Constat initial

Chaque observation Oracle était déjà stockée avec sa hauteur, mais `ComputeTWAP` ignorait cette information et calculait une moyenne arithmétique. Un prix resté valable 40 blocs avait donc le même poids qu'un prix observé pendant 5 blocs. En cas de tallies irréguliers ou manqués, le résultat ne représentait plus le prix réellement en vigueur pendant la fenêtre.

Par exemple, avec un prix de `1` pendant 40 blocs puis de `2` pendant 5 blocs :

```text
moyenne arithmétique précédente = (1 + 2) / 2 = 1,5
TWAP réel                     = (1 × 40 + 2 × 5) / 45 = 1,111…
```

Le second défaut était plus direct : si aucun historique n'était disponible, le gestionnaire de swap ignorait volontairement le contrôle TWAP et autorisait la transaction. Un Market fraîchement activé, restauré sans historique ou privé de données suffisantes bénéficiait donc de moins de protection que le régime normal.

##### Solution implémentée

Le calcul traite maintenant les observations comme une fonction en escalier sur l'intervalle exact :

```text
[hauteur actuelle − 45, hauteur actuelle)
```

Pour chaque segment, le prix est multiplié par le nombre exact de blocs pendant lesquels il est resté valable. La somme est ensuite divisée par 45. Une observation publiée à la hauteur actuelle n'a donc aucun poids rétroactif ; elle commencera à compter au bloc suivant.

Lors du nettoyage du stockage, le Market conserve aussi le dernier prix situé à la frontière ou avant celle-ci. Cette observation est indispensable pour connaître le prix en vigueur au début de la fenêtre, même si le tally précédent est plus ancien en raison d'une irrégularité.

La présence d'au moins un snapshot ne suffit plus. Chaque entrée Oracle nécessaire à la paire doit posséder une observation située à la frontière de la fenêtre ou avant elle. Pour LUNC/USTC, cette règle s'applique séparément à `uusd` et `UST`. Si une seule entrée ne couvre que 44 blocs, le swap retourne l'erreur Market 11, `complete TWAP history is not available`, avant toute mutation du pool ou des soldes.

La première activation post-upgrade dispose normalement d'une époque complète pour accumuler cet historique. Dans tous les autres cas, le comportement est fermé par défaut : après une restauration ne contenant pas les snapshots, les swaps attendent simplement 45 blocs d'observations avant de reprendre. Un redémarrage normal du même stockage conserve les snapshots et ne provoque pas cette attente.

##### Tests de régression

Les tests ajoutés ou adaptés vérifient :

- le résultat exact `50 / 45` pour les durées irrégulières 40 blocs à `1` puis 5 blocs à `2` ;
- la conservation du snapshot antérieur à la frontière lors du nettoyage ;
- le refus d'un historique de seulement 44 blocs et son acceptation au 45e bloc ;
- le refus atomique d'un swap lorsque `UST` couvre 45 blocs mais `uusd` seulement 44 ;
- la réussite du même swap un bloc plus tard, lorsque les deux fenêtres sont complètes ;
- les déviations supérieures et inférieures à 10 % ;
- les chemins de swap natif, `SwapSend`, Wasm, No-Mint, plafond journalier, première activation et actif générique.

#### IMP-007 — Résolution de GAP-004 : frein de gouvernance existant et voie accélérée utilisable

**Statut initial : PARTIEL, PUIS REQUALIFIÉ**
**Statut actualisé : RESOLVED LOCAL — TESTS PASS**
**Gravité initiale : élevée / P1**

##### Constat initial

L'analyse initiale cherchait un booléen ou une commande dédiée permettant à la gouvernance d'arrêter le Market. Ce nouvel interrupteur n'est pas nécessaire. Le paramètre existant `MinStabilitySpread` est déjà enregistré dans le sous-espace `market` et modifiable par la route de gouvernance des changements de paramètres. Les forks historiques de l'application l'ont réglé à `1`, soit 100 %, précisément pour neutraliser les swaps. Une requête sur l'état Columbus-5 du 8 août 2026 a également retourné cette valeur.

À 100 %, le frais calculé est égal à la sortie brute. La sortie nette du trader devient donc nulle et le message échoue avec `zero swap coin`. Remettre le spread approuvé à `0,0035` réouvre le chemin de swap, sous réserve que l'activation de base, l'Oracle, le TWAP, le plafond et la liquidité soient eux aussi valides. Ce frein est distinct de l'auto-kill Oracle et de la première activation : aucun de ces mécanismes ne peut rouvrir l'un des autres.

Le contrôle a toutefois révélé un défaut de configuration dans la gouvernance accélérée. Le seuil existe à `0,667` et sa période est plus courte, mais le dépôt minimal accéléré hérité du SDK était libellé en `stake`, tandis que le genesis personnalisé ne convertissait que le dépôt ordinaire en `uluna`. Une proposition accélérée ne pouvait donc pas être financée avec le token natif de Terra Classic.

##### Solution implémentée

Aucun nouvel état d'arrêt n'a été ajouté au Market. La solution conserve le mécanisme historique afin d'éviter deux sources de vérité concurrentes :

- `MinStabilitySpread = 1` ferme les swaps ;
- `MinStabilitySpread = 0,0035` rétablit le spread MM2 approuvé ;
- la validation de la sortie nulle, des frais et de la liquidité est maintenant exécutée avant toute modification du delta du pool ou des soldes ;
- le genesis de gouvernance utilise `uluna` pour les dépôts ordinaires et accélérés ;
- la migration v15 convertit un éventuel denom SDK `stake` en `uluna` sur les deux chemins, tout en conservant les montants, les périodes et le seuil accéléré `0,667`.

##### Tests de régression et preuves réseau

Les tests Keeper ferment puis rouvrent successivement :

- un swap LUNC vers USTC ;
- un swap USTC vers LUNC ;
- un `SwapSend` LUNC vers USTC.

Dans l'état fermé, le trader, le destinataire, le compte Market et `TerraPoolDelta` restent strictement inchangés. Le même message réussit après retour à `0,0035`. Un export/import de genesis conserve également la valeur fermée à `1`.

Un test d'intégration de l'application complète soumet ensuite deux véritables propositions accélérées, finance chacune avec le dépôt requis en `uluna`, enregistre un vote pondéré et exécute le tally : la première ferme le Market et la seconde le réouvre. Il vérifie le seuil `0,667`, le statut `PASSED` et la modification du paramètre uniquement après adoption. Les tests de migration prouvent séparément la conversion idempotente du denom de dépôt.

Sur le devnet persistant, deux propositions ordinaires ont validé le chemin réseau complet de modification du paramètre :

| Proposition | Action | Résultat | Transaction de soumission |
|---|---|---|---|
| 1 | passage de `0,0035` à `1` | `PASSED`, puissance `YES` : `900 000 000 000` | `344A0A20…90DB` |
| 2 | retour de `1` à `0,0035` | `PASSED`, puissance `YES` : `900 000 000 000` | `FEAEC621…E614` |

Le nœud a été redémarré après la proposition 2 et a conservé `0,0035`. La voie accélérée n'a pas été rejouée sur ce volume ancien, car son état avait précisément conservé le dépôt erroné en `stake` ; elle est couverte par le test d'intégration complet et par la migration v15 corrigée. Une répétition multi-validateur devra encore vérifier la frontière exacte du tally : `0,667` est la valeur SDK employée pour représenter la règle de deux tiers.

### 10. Améliorations restant à réaliser

Les écarts de conception GAP-001 à GAP-004 sont tous traités localement. Il ne reste pas de mécanisme MM2 identifié comme absent dans cette série. Les limites encore ouvertes concernent la publication, l'upgrade depuis un snapshot réel, le harnais multi-validateur et la validation économique longue décrits ci-dessous.

### 11. Limites de la campagne

Le harnais multi-validateur de base est corrigé. Les points suivants doivent encore être exercés en étendant ses scénarios ou dans un environnement public :

1. revalidation E2E de l'auto-kill Oracle avec au moins quatre validateurs de puissances inégales et pertes successives de 25 blocs ;
2. revalidation en réseau du TWAP avec votes irréguliers et changement de majorité ;
3. répétition de l'upgrade corrigé depuis un snapshot pré-v15 réaliste, puis activation à l'époque suivante ;
4. charge réellement parallèle depuis plusieurs comptes et plusieurs proposers ;
5. campagne économique longue couvrant plusieurs époques de production simulées.
6. fermeture et réouverture accélérées sur plusieurs validateurs, avec contrôle de la frontière du seuil `0,667`.

Ces tests restants ne changent pas le verdict courant : les défauts P0 sont déjà reproductibles ou déterministes par lecture du chemin d'exécution.

### 12. Recommandations priorisées

#### P0 — avant tout test communautaire

1. Publier le correctif validé du feeder pour `UST` (`USD par USTC`) et conserver le test d'intégration réciproque avec le core.
2. Publier le correctif validé d'`InitGenesis` et de l'upgrade garantissant un véritable `ModuleAccount` `market_accumulator`, y compris en cas de collision d'adresse.
3. Publier la migration v15 idempotente validée localement et répéter l'essai sur un snapshot pré-v15 représentatif.
4. Conserver les tests bloquants qui imposent MM désactivé pendant une époque complète et deux réserves alimentées avant la première activation.

#### P1 — avant proposition mainnet

1. Publier l'implémentation locale de `base_pool`/PRP avec la séparation 7 % adaptatif / 10 % plafond strict et la rejouer sur plusieurs époques.
2. Publier l'auto-kill quorum local de 25 blocs, puis valider son arrêt persistant et son réarmement sur un réseau multi-validateur.
3. Publier le TWAP pondéré local et confirmer en réseau le blocage pendant l'amorçage puis la reprise au 45e bloc.
4. Publier la normalisation du dépôt accéléré en `uluna`, puis rejouer la fermeture et la réouverture par `MinStabilitySpread` sur le harnais multi-validateur.
5. Étendre le harnais multi-validateur réparé avec des assertions bloquantes pour les pannes de quorum et les transitions de gouvernance.

#### P2 — qualité d'exploitation

1. Fournir des scripts reproductibles de charge multi-comptes, d'export/import et de collecte des preuves.
2. Documenter clairement les valeurs mainnet, testnet et devnet pour éviter qu'une époque de 100 blocs ne soit réutilisée hors local.

### 13. Critères de reprise de la validation

Une nouvelle campagne pourra conclure « GO testnet communautaire » lorsque :

- les trois P0 possèdent chacun un test de régression ; leurs correctifs locaux doivent être revus et publiés ;
- un upgrade pré-v15 produit des blocs, taxes et swaps sans panic avec MM initialement désactivé ;
- les cotations LUNC/USTC correspondent aux deux prix USD bruts dans les deux sens ;
- le réseau multi-validateur démontre arrêt, persistance et reprise sous les seuils de quorum ;
- le réseau multi-validateur démontre une fermeture accélérée à 100 %, sa persistance et une réouverture à 0,35 % avec dépôts `uluna` ;
- la séparation 7 % adaptatif / 10 % plafond strict est documentée, l'adaptation de liquidité satisfait les bornes de la proposition sur plusieurs époques et le TWAP est réellement pondéré.

### 14. Confidentialité et reproductibilité

Ce document ne contient aucune phrase mnémonique, clé privée ou secret Docker. Les adresses et transactions de la chaîne locale sont uniquement des données de test. La clé du validateur local utilisée pour le smoke test d'import a été copiée dans `/private/tmp`, montée en lecture seule, puis supprimée. Les exports de test restent hors du dépôt.

Le devnet Docker est externe au dépôt MM2. Le nœud de la campagne initiale utilisait l'image construite depuis le commit core figé ; seule l'image du feeder avait alors été reconstruite. Les correctifs core sont maintenant clairement identifiés dans la branche locale avec leurs tests de régression et ne sont pas encore publiés.

### 15. Conclusion finale

Le cœur No-Mint démontre une base prometteuse : transferts depuis un pool réel, absence de mint Market, burn des frais et des fins d'époque, fiscalité à 60 %, plafonds et refus atomiques fonctionnent dans le profil local. Les suites Go et le fuzzing n'ont révélé aucune régression générale.

Le code n'est cependant pas encore prêt pour un test communautaire public. `INT-001`, `INT-002`, `INT-003` et `GAP-001` à `GAP-004` sont corrigés ou requalifiés puis validés localement, mais restent à relire, séparer en changements révisables et publier. Le chemin Oracle/TWAP/Market fonctionne désormais sur quatre validateurs locaux, sans pour autant valider encore les pertes de quorum, les puissances inégales et les transitions de gouvernance. L'exemple à 10 % de la proposition devrait également être clarifié.

**Décision recommandée : NO-GO communautaire en l'état.** La prochaine étape raisonnable est la revue et la publication des correctifs locaux, un test d'upgrade sur snapshot, puis l'extension de la campagne multi-validateur aux scénarios de panne et de gouvernance. Ce rapport fournit la baseline et les critères permettant de mesurer cette progression sans ambiguïté.
