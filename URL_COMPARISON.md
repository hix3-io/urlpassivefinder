# Comparaison des URLs trouvées pour entities.fr

## Nombre total d'URLs par outil

| Outil | Nombre d'URLs |
|-------|--------------|
| **URLPassiveFinder v0.3.0** | 2,837 |
| **gau** | 969 |
| **urlfinder** | 9,903 |
| **waybackurls** | 10,518 |

## Analyse de similarité

### Intersections entre outils

| Comparaison | URLs communes |
|-------------|---------------|
| URLPassiveFinder ∩ gau | 203 |
| URLPassiveFinder ∩ urlfinder | 2,708 |
| gau ∩ urlfinder | 939 |

### URLs uniques

| Outil | URLs uniques vs urlfinder |
|-------|---------------------------|
| URLPassiveFinder | 129 URLs que urlfinder n'a pas |
| urlfinder | 7,195 URLs que URLPassiveFinder n'a pas |

## Analyse des différences

### Pourquoi urlfinder trouve plus d'URLs ?

1. **Moins de déduplication agressive**: Notre outil normalise les URLs (ports, params triés, etc.)
2. **Gestion des variantes**: urlfinder garde les URLs avec différents paramètres
3. **Subdomains**: urlfinder trouve plus de subdomains (20+ vs notre outil)

### Exemples d'URLs manquées par URLPassiveFinder:
- URLs avec paramètres de tracking différents
- URLs d'images avec dimensions variées (jpg?crop=1&valign=Middle)
- URLs de CDN et d'API avec variations

## Conclusion

- **gau**: Trouve le moins d'URLs (969), couverture limitée
- **URLPassiveFinder**: Trouve 2,837 URLs, bon équilibre mais trop de déduplication
- **urlfinder**: Trouve 9,903 URLs, meilleure couverture mais plus de doublons
- **waybackurls**: Trouve le plus d'URLs (10,518), approche exhaustive

Notre outil partage 95% (2,708/2,837) de ses URLs avec urlfinder, mais manque 7,195 URLs supplémentaires.
