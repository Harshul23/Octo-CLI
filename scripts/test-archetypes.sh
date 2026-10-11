#!/usr/bin/env bash
# ==============================================================================
# Octo Dynamic Archetype Validation Suite
# Automatically discovers and validates all repository archetypes in a target directory.
# ==============================================================================

set -uo pipefail

TARGET_DIR="${1:-/Users/harshul/Projects/Octo-testing}"
OCTO_BIN="${OCTO_BIN:-octo}"

# Color codes
GREEN="\033[0;32m"
BLUE="\033[0;34m"
YELLOW="\033[1;33m"
RED="\033[0;31m"
CYAN="\033[0;36m"
BOLD="\033[1m"
RESET="\033[0m"

echo -e "\n${BOLD}${BLUE}🐙 Octo Dynamic Archetype Test Suite${RESET}"
echo -e "══════════════════════════════════════════════════════════════════════"
echo -e "Target Directory: ${CYAN}${TARGET_DIR}${RESET}"
echo -e "Using Octo Binary: ${CYAN}$(which "$OCTO_BIN" 2>/dev/null || echo "$OCTO_BIN")${RESET}"

if [ ! -d "$TARGET_DIR" ]; then
  echo -e "${RED}Error: Directory '$TARGET_DIR' does not exist.${RESET}\n"
  exit 1
fi

# Discover all candidate subdirectories
REPOS=()
while IFS= read -r dir; do
  if [ -d "$dir" ] && [ "$dir" != "$TARGET_DIR" ]; then
    # Check if directory has any manifest or source indicators
    if [ -f "$dir/package.json" ] || [ -f "$dir/go.mod" ] || [ -f "$dir/Cargo.toml" ] || \
       [ -f "$dir/pyproject.toml" ] || [ -f "$dir/requirements.txt" ] || [ -f "$dir/docker-compose.yml" ]; then
      REPOS+=("$dir")
    fi
  fi
done < <(find "$TARGET_DIR" -mindepth 1 -maxdepth 1 -type d | sort)

TOTAL_COUNT=${#REPOS[@]}
if [ "$TOTAL_COUNT" -eq 0 ]; then
  echo -e "${YELLOW}Warning: No valid repository archetypes found in '$TARGET_DIR'.${RESET}\n"
  exit 0
fi

echo -e "Discovered Repos: ${BOLD}${TOTAL_COUNT}${RESET} dynamic archetypes"
echo -e "──────────────────────────────────────────────────────────────────────\n"

PASSED=0
FAILED=0
START_TIME=$(date +%s)

printf "%-30s | %-12s | %-12s | %-6s | %-6s | %-8s\n" "ARCHETYPE" "LANGUAGE" "FRAMEWORK" "STEPS" "CONF" "STATUS"
printf "%s\n" "-------------------------------+--------------+--------------+--------+--------+---------"

for REPO_PATH in "${REPOS[@]}"; do
  REPO_NAME=$(basename "$REPO_PATH")

  # 1. Inspect repository
  INSPECT_JSON=$("$OCTO_BIN" inspect --json "$REPO_PATH" 2>/dev/null || true)
  if [ -z "$INSPECT_JSON" ]; then
    printf "%-30s | %-12s | %-12s | %-6s | %-6s | ${RED}%-8s${RESET}\n" "$REPO_NAME" "-" "-" "-" "-" "FAIL (inspect)"
    ((FAILED++))
    continue
  fi

  LANG=$(echo "$INSPECT_JSON" | grep -o '"language": *"[^"]*"' | head -1 | cut -d'"' -f4 || echo "unknown")
  FRAMEWORK=$(echo "$INSPECT_JSON" | grep -o '"framework": *"[^"]*"' | head -1 | cut -d'"' -f4 || echo "unknown")
  CONF_RAW=$(echo "$INSPECT_JSON" | grep -o '"confidence": *[0-9.]*' | head -1 | awk '{print $2}' || echo "0")
  CONF_PCT=$(awk -v c="$CONF_RAW" 'BEGIN { printf "%.0f%%", c*100 }' 2>/dev/null || echo "100%")

  # 2. Plan execution
  PLAN_JSON=$("$OCTO_BIN" plan --json "$REPO_PATH" 2>/dev/null || true)
  STEP_COUNT=$(echo "$PLAN_JSON" | grep -o '"id":' | wc -l | tr -d ' ' || echo "0")
  
  if [ "$STEP_COUNT" -eq 0 ]; then
    printf "%-30s | %-12s | %-12s | %-6s | %-6s | ${RED}%-8s${RESET}\n" "$REPO_NAME" "$LANG" "$FRAMEWORK" "0" "$CONF_PCT" "FAIL (plan)"
    ((FAILED++))
    continue
  fi

  # 3. Preview machine impact
  PREVIEW_ERR=0
  "$OCTO_BIN" preview "$REPO_PATH" >/dev/null 2>&1 || PREVIEW_ERR=1
  if [ "$PREVIEW_ERR" -ne 0 ]; then
    printf "%-30s | %-12s | %-12s | %-6s | %-6s | ${RED}%-8s${RESET}\n" "$REPO_NAME" "$LANG" "$FRAMEWORK" "$STEP_COUNT" "$CONF_PCT" "FAIL (preview)"
    ((FAILED++))
    continue
  fi

  # 4. Environment analysis
  "$OCTO_BIN" env "$REPO_PATH" >/dev/null 2>&1 || true

  printf "%-30s | %-12s | %-12s | %-6s | %-6s | ${GREEN}%-8s${RESET}\n" "$REPO_NAME" "$LANG" "$FRAMEWORK" "$STEP_COUNT" "$CONF_PCT" "PASS"
  ((PASSED++))
done

END_TIME=$(date +%s)
ELAPSED=$((END_TIME - START_TIME))

echo -e "──────────────────────────────────────────────────────────────────────"
if [ "$FAILED" -eq 0 ]; then
  echo -e "${GREEN}${BOLD}✓ All ${PASSED}/${TOTAL_COUNT} Archetypes Passed successfully in ${ELAPSED}s (100% Reliability)${RESET}\n"
  exit 0
else
  echo -e "${RED}${BOLD}✗ ${FAILED}/${TOTAL_COUNT} Archetypes Failed validation (Elapsed: ${ELAPSED}s)${RESET}\n"
  exit 1
fi
