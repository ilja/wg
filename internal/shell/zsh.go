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

  command %s "$@"
}
`, binaryName, binaryName)
}
