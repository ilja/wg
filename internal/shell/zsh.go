package shell

import "fmt"

func ZshInit(binaryName string) string {
	return fmt.Sprintf(`wg() {
  if [[ "$1" == "switch" ]]; then
    shift
    local wg_switch_path
    wg_switch_path="$(command %s switch --path-output "$@")"
    local wg_switch_status=$?
    if [[ $wg_switch_status -ne 0 ]]; then
      return $wg_switch_status
    fi
    if [[ -z "$wg_switch_path" ]]; then
      print -u2 "wg switch returned empty path"
      return 1
    fi
    builtin cd -- "$wg_switch_path"
    return $?
  fi

  if [[ "$1" == "remove" ]]; then
    shift
    local wg_remove_cd_target
    wg_remove_cd_target="$(command %s remove --print-cd-target "$@")"
    local wg_remove_status=$?
    if [[ $wg_remove_status -ne 0 ]]; then
      return $wg_remove_status
    fi
    if [[ -n "$wg_remove_cd_target" ]]; then
      builtin cd -- "$wg_remove_cd_target"
      return $?
    fi
    return 0
  fi

  command %s "$@"
}

_wg() {
  if [[ "${words[2]}" != "remove" ]]; then
    return 1
  fi

  local wg_target_index=0
  local wg_word_index=3
  while (( wg_word_index <= CURRENT )); do
    local wg_word="${words[$wg_word_index]}"
    if [[ "$wg_word" != -* ]]; then
      wg_target_index=$wg_word_index
      break
    fi
    (( wg_word_index++ ))
  done

  if [[ $wg_target_index -eq 0 || $wg_target_index -ne $CURRENT ]]; then
    return 0
  fi

  local wg_remove_output
  wg_remove_output="$(command %s config shell complete remove -- "${words[$CURRENT]}" 2>/dev/null)" || return 0
  if [[ -n "$wg_remove_output" ]]; then
    local -a wg_remove_candidates
    wg_remove_candidates=("${(@f)wg_remove_output}")
    compadd -a wg_remove_candidates
  fi
  return 0
}

if (( $+functions[compdef] )); then
  compdef _wg wg
fi
`, binaryName, binaryName, binaryName, binaryName)
}
