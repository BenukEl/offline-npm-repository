#!/bin/bash

# Vérifie que le fichier d'entrée est fourni
if [ $# -lt 1 ]; then
  echo "Usage: $0 dependencies_file"
  exit 1
fi

DEPENDENCIES_FILE=$1
OUTPUT_FILE="last_downloaded_versions.txt" 

# Vérifie que le fichier d'entrée existe
if [ ! -f "$DEPENDENCIES_FILE" ]; then
  echo "Le fichier $DEPENDENCIES_FILE n'existe pas."
  exit 1
fi

# Efface le fichier de sortie s'il existe déjà
> "$OUTPUT_FILE"

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
  versions=$(echo "$package_metadata" | jq -r '.versions | keys[]' | grep -E '^[0-9]+\.[0-9]+\.[0-9]+$' | sort -V)

  if [[ -z "$versions" ]]; then
    echo "Aucune version valide trouvée pour $package_name"
    return 1
  fi

  echo "Versions trouvées pour $package_name (triées) :"
  echo "$versions"

  # Nettoyer le cache npm avant les installations
  npm cache clean --force

  # Traiter chaque version
  local last_version=""
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
    last_version="$version"
  done

  # Enregistrer la dernière version dans le fichier
  if [[ -n "$last_version" ]]; then
    echo "$package_name@$last_version" >> "$OUTPUT_FILE"
  fi
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
echo "Dernières versions téléchargées enregistrées dans $OUTPUT_FILE"
