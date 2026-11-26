#!/bin/sh
# Instala os comandos `nota` e `nota-add` para notas.lauraesteves.com.
#
#   curl -fsSL https://notas.lauraesteves.com/install.sh | sh
#
# Detecta o shell pelo $SHELL (bash ou zsh). Para escolher outro:
#   curl -fsSL https://notas.lauraesteves.com/install.sh | sh -s -- zsh
#
# Uso depois de instalar:
#   nota netbook                  mostra a nota
#   nota-add netbook "texto"      adiciona uma linha no final
#   nota-add netbook              adiciona o que for colado (Ctrl+D para terminar)
#
# Rodar de novo atualiza a instalação: o bloco entre os marcadores é trocado, nunca duplicado.
# Cada shell tem o próprio arquivo de inicialização, então instalar no bash e depois no zsh
# só cria um bloco em cada um.

set -eu

BEGIN_MARK='# >>> notas.lauraesteves.com >>>'
END_MARK='# <<< notas.lauraesteves.com <<<'

say() { printf '%s\n' "$*"; }
warn() { printf 'aviso: %s\n' "$*" >&2; }
die() { printf 'erro: %s\n' "$*" >&2; exit 1; }

block() {
	cat <<'EOF'
# >>> notas.lauraesteves.com >>>
# Gerado por https://notas.lauraesteves.com/install.sh (rode de novo para atualizar).
NOTAS_URL="${NOTAS_URL:-https://notas.lauraesteves.com/raw}"
unalias nota nota-add 2>/dev/null || true

nota() {
  if [ $# -ne 1 ]; then
    echo 'uso: nota <nota>' >&2
    return 2
  fi
  curl -sS "$NOTAS_URL/$1"
}

nota-add() {
  if [ $# -lt 1 ]; then
    echo 'uso: nota-add <nota> "texto"   (sem texto: cole e termine com Ctrl+D)' >&2
    return 2
  fi
  local url="$NOTAS_URL/$1" resp code current
  shift
  # Lê a nota atual com o status no final para não sobrescrever tudo se a leitura falhar.
  resp=$(curl -sS -w '\n%{http_code}' "$url") || return 1
  code=${resp##*$'\n'}
  current=${resp%$'\n'*}
  case "$code" in
    200) ;;
    404) current='' ;;
    *) printf 'nota-add: leitura falhou (HTTP %s): %s\n' "$code" "${current%$'\n'}" >&2; return 1 ;;
  esac
  {
    printf '%s' "$current"
    if [ $# -gt 0 ]; then printf '%s\n' "$*"; else cat; fi
  } | curl -sS -X PUT -H 'Content-Type: text/plain; charset=utf-8' --data-binary @- "$url"
}
# <<< notas.lauraesteves.com <<<
EOF
}

main() {
	command -v curl >/dev/null 2>&1 || die 'curl não encontrado. Instale o curl e rode de novo.'

	shell_name=${1:-${SHELL:-}}
	shell_name=${shell_name##*/}

	case "$shell_name" in
	zsh)
		rc="${ZDOTDIR:-$HOME}/.zshrc"
		;;
	bash)
		# No macOS o Terminal abre login shells, que leem ~/.bash_profile e não ~/.bashrc.
		if [ "$(uname -s)" = Darwin ]; then
			rc="$HOME/.bash_profile"
		else
			rc="$HOME/.bashrc"
		fi
		;;
	*)
		die "shell não suportado: '${shell_name:-desconhecido}'. Use bash ou zsh: curl -fsSL https://notas.lauraesteves.com/install.sh | sh -s -- bash"
		;;
	esac

	touch "$rc"
	tmp=$(mktemp "${TMPDIR:-/tmp}/notas-install.XXXXXX")
	trap 'rm -f "$tmp"' EXIT

	if grep -qxF "$BEGIN_MARK" "$rc"; then
		action='atualizado'
	else
		action='instalado'
	fi

	# Copia o arquivo sem o bloco antigo (se houver) e sem linhas em branco no final, para
	# reinstalar não acumular espaço. O bloco novo vai para o final, assim sempre vence
	# qualquer definição anterior.
	awk -v b="$BEGIN_MARK" -v e="$END_MARK" '
		$0 == b { skip = 1; next }
		$0 == e { skip = 0; next }
		skip { next }
		/^[[:space:]]*$/ { blank++; next }
		{ while (blank > 0) { print ""; blank-- } print }
	' "$rc" >"$tmp"

	# Definições feitas à mão fora do bloco (por exemplo, uma versão antiga com a nota fixa).
	old=$(grep -nE '^[[:space:]]*((function[[:space:]]+)?nota(-add)?[[:space:]]*\(\)|alias[[:space:]]+nota(-add)?=)' "$tmp" || true)

	cp "$rc" "$rc.notas.bak"
	{
		cat "$tmp"
		if [ -s "$tmp" ]; then printf '\n'; fi
		block
	} >"$rc"

	say "nota e nota-add $action em $rc (backup em $rc.notas.bak)."

	if [ -n "$old" ]; then
		warn "$rc já tinha outras definições de nota/nota-add:"
		printf '%s\n' "$old" | sed 's/^/  linha /' >&2
		warn 'o bloco novo fica no final e as substitui, mas vale apagar essas linhas.'
	fi

	if path=$(command -v nota 2>/dev/null) && [ -n "$path" ] && [ "${path#/}" != "$path" ]; then
		warn "existe um programa 'nota' em $path; a função nova tem prioridade sobre ele no $shell_name."
	fi

	say ''
	say "Para usar agora: source $rc   (ou abra um terminal novo)"
	say 'Exemplos:'
	say '  nota netbook'
	say '  nota-add netbook "comprar café"'
}

main "$@"
