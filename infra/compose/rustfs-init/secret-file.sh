carriage_return=$(printf '\r')

read_secret() {
  secret_file=$1
  secret_name=$2
  [ -f "$secret_file" ] || die "$secret_name is not a regular file"
  [ -r "$secret_file" ] || die "$secret_name is unreadable"
  snapshot=$(mktemp /tmp/rustfs-secret.XXXXXX) || die "$secret_name cannot be read"
  trap 'rm -f "$snapshot"' 0
  trap 'exit 1' 1 2 3 15
  if ! cat "$secret_file" >"$snapshot"; then
    rm -f "$snapshot"
    die "$secret_name cannot be read"
  fi
  invalid_byte_count=$(LC_ALL=C tr -d '\041-\176\015\012' <"$snapshot" | wc -c | tr -d '[:space:]')
  case "$invalid_byte_count" in
    0) ;;
    *)
      rm -f "$snapshot"
      die "$secret_name contains an invalid character"
      ;;
  esac
  exec 3<"$snapshot" || {
    rm -f "$snapshot"
    die "$secret_name cannot be read"
  }
  value=
  if IFS= read -r value <&3; then
    :
  elif [ -n "$value" ]; then
    :
  else
    exec 3<&-
    rm -f "$snapshot"
    die "$secret_name is empty"
  fi
  trailing_value=
  if IFS= read -r trailing_value <&3 || [ -n "$trailing_value" ]; then
    exec 3<&-
    rm -f "$snapshot"
    die "$secret_name contains an invalid character"
  fi
  exec 3<&-
  rm -f "$snapshot"
  trap - 0 1 2 3 15
  case "$value" in *"$carriage_return") value=${value%"$carriage_return"};; esac
  [ -n "$value" ] || die "$secret_name is empty"
  case "$value" in *[![:graph:]]*) die "$secret_name contains an invalid character";; esac
  printf '%s' "$value"
}
