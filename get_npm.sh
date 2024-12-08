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

# Crée le fichier des dernières versions téléchargées s'il n'existe pas
if [ ! -f "$OUTPUT_FILE" ]; then
  touch "$OUTPUT_FILE"
fi

# Fonction pour encoder une dépendance pour l'URL
url_encode() {
  local raw=$1
  echo -n "$raw" | jq -sRr @uri
}

# Fonction pour installer et traiter les versions d'une dépendance
process_package_versions() {
  local package_name=$1

  echo "======================================="
  echo "Traitement du package : $package_name"
  echo "======================================="

  # Encoder le nom de la dépendance pour l'utiliser dans l'URL
  local encoded_name
  encoded_name=$(url_encode "$package_name")

  # Récupérer les métadonnées du package via npm registry
  local package_metadata
  package_metadata=$(curl -s "https://registry.npmjs.org/$encoded_name")
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

  echo "Versions disponibles pour $package_name (triées) :"
  echo "$versions"

  # Trouver la dernière version téléchargée pour cette dépendance
  local last_downloaded_version
  last_downloaded_version=$(grep "^$package_name@" "$OUTPUT_FILE" | cut -d'@' -f2)

  echo "Dernière version téléchargée pour $package_name : $last_downloaded_version"

  # Filtrer les versions à télécharger
  local versions_to_download
  if [[ -z "$last_downloaded_version" ]]; then
    # Si aucune version n'a été téléchargée, télécharger toutes les versions
    versions_to_download=$versions
  else
    # Télécharger uniquement les versions supérieures à la dernière téléchargée
    versions_to_download=$(echo "$versions" | awk -v last="$last_downloaded_version" '{
      if ($0 ~ last) { seen=1; next }
      if (seen) print $0
    }')
  fi

  if [[ -z "$versions_to_download" ]]; then
    echo "Aucune nouvelle version à télécharger pour $package_name."
    return 0
  fi

  echo "Versions à télécharger pour $package_name :"
  echo "$versions_to_download"

  # Nettoyer le cache npm avant les installations
  npm cache clean --force

  # Traiter chaque version
  local latest_version=""
  for version in $versions_to_download; do
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
    latest_version="$version"
  done

  # Mettre à jour la dernière version dans le fichier
  if [[ -n "$latest_version" ]]; then
    # Supprimer l'entrée existante pour ce package
    sed -i "/^$package_name@/d" "$OUTPUT_FILE"
    # Ajouter la nouvelle version
    echo "$package_name@$latest_version" >> "$OUTPUT_FILE"
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
echo "Les dernières versions téléchargées ont été mises à jour dans $OUTPUT_FILE"
