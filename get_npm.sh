#!/bin/bash

# Vérifie que le fichier d'entrée est fourni
if [ $# -lt 1 ]; then
  echo "Usage: $0 dependencies_file"
  exit 1
fi

DEPENDENCIES_FILE=$1

# Vérifie que le fichier d'entrée existe
if [ ! -f "$DEPENDENCIES_FILE" ]; then
  echo "Le fichier $DEPENDENCIES_FILE n'existe pas."
  exit 1
fi

# Fonction pour installer et traiter les versions d'une dépendance
process_package_versions() {
  local package_name=$1

  echo "======================================="
  echo "Traitement du package : $package_name"
  echo "======================================="

  # Récupérer les métadonnées du package via npm registry
  local package_metadata
  package_metadata=$(curl -s "https://registry.npmjs.org/$package_name")
  if [[ -z "$package_metadata" ]]; then
    echo "Erreur : Impossible de récupérer les métadonnées pour $package_name"
    return 1
  fi

  # Extraire les versions valides au format x.y.z
  local versions
  versions=$(echo "$package_metadata" | jq -r '.versions | keys[]' | grep -E '^[0-9]+\.[0-9]+\.[0-9]+$')

  if [[ -z "$versions" ]]; then
    echo "Aucune version valide trouvée pour $package_name"
    return 1
  fi

  echo "Versions trouvées pour $package_name : $versions"

  # Traiter chaque version
  for version in $versions; do
    echo "Installation de $package_name@$version..."

    # Supprimer node_modules et installer la version
    rm -rf node_modules
    npm install "$package_name@$version" --silent

    # Vérifier si l'installation a réussi
    if [ $? -ne 0 ]; then
      echo "Erreur lors de l'installation de $package_name@$version"
      continue
    fi

    echo "Installation réussie : $package_name@$version"

    # Nettoyer le cache npm après chaque installation
    npm cache clean --force
  done
}

# Configure npm pour utiliser le registre local
# Ancien registry: https://registry.npmjs.org/
OLD_REGISTRY=$(npm get registry)
npm set registry http://localhost:4873

# Lire le fichier d'entrée et traiter chaque dépendance
while IFS= read -r dependency; do
  # Ignore les lignes vides ou commentaires
  if [[ -z "$dependency" || "$dependency" == \#* ]]; then
    continue
  fi

  # Appeler la fonction pour traiter le package
  process_package_versions "$dependency"
done < "$DEPENDENCIES_FILE"

# Restaurer l'ancien registre npm
npm set registry "$OLD_REGISTRY"

echo "Traitement terminé pour toutes les dépendances."

# Génération du fichier de liste des nouveaux fichiers dans verdaccio/storage

# Création du dossier deps_list s'il n'existe pas
mkdir -p deps_list

# Nom du fichier avec timestamp
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
NEW_FILE="deps_list/deps_$TIMESTAMP.txt"

# Trouver tous les fichiers dans verdaccio/storage
ALL_FILES=$(mktemp)
find verdaccio/storage/ -type f > "$ALL_FILES"

# Fusionner les fichiers existants dans deps_list pour les exclure
EXISTING_FILES=$(mktemp)
cat deps_list/deps_*.txt > "$EXISTING_FILES" 2>/dev/null

# Filtrer les fichiers nouveaux
grep -Fxv -f "$EXISTING_FILES" "$ALL_FILES" > "$NEW_FILE"

# Nettoyer les fichiers temporaires
rm -f "$ALL_FILES" "$EXISTING_FILES"

echo "Nouvelle liste des fichiers créée : $NEW_FILE"