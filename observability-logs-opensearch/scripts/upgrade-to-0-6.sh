#!/usr/bin/env bash
# Migrate existing container-logs-* indices onto the 0.6.x mapping.
# Run AFTER `helm upgrade … observability-logs-opensearch --version 0.6.0`.
#
# 0.6.0 stores kubernetes.labels as a flat_object (every pod label is searchable, not a
# fixed allowlist of 12) and maps openchoreo_cluster_instance. OpenSearch cannot change
# an existing field from an object to flat_object, so each index is rebuilt under its
# own name - the adapter addresses indices by date, so the name must not change:
#
#   1. block writes on <index> and copy it into migrate-0-6-<index>
#   2. verify the copy, delete <index>, recreate it from the 0.6.0 index template
#   3. copy the documents back and delete migrate-0-6-<index>
#
# Documents keep their _id, so copying back is idempotent. The script runs after the
# upgrade because it reads the mapping from the registered 0.6.0 template.
#
# Things to know before running:
# - Today's (UTC) index is skipped: Fluent Bit is still writing to it and would drop
#   records rejected by the write block. Re-run the script tomorrow to migrate it.
# - While an index is being rebuilt, logs for that day are missing from queries.
# - OpenSearch does not allow setting index.creation_date, so a rebuilt index restarts
#   its ISM retention clock and is kept up to one retention period longer than usual.
# - Rebuilding needs free disk space for one extra copy of the largest index.
#
# Safe to re-run: migrated indices are skipped, and an interrupted run is resumed from
# whatever step it stopped at.

set -euo pipefail

NS="${NS:-openchoreo-observability-plane}"
OS_SECRET="${OS_SECRET:-opensearch-admin-credentials}"
OS_SERVICE="${OS_SERVICE:-svc/opensearch}"
LOCAL_PORT="${LOCAL_PORT:-9200}"
INDEX_PREFIX="${INDEX_PREFIX:-container-logs-}"
TEMPLATE_NAME="${TEMPLATE_NAME:-container-logs}"
TMP_PREFIX="migrate-0-6-"

C_BOLD=$'\033[1m'; C_DIM=$'\033[2m'; C_GREEN=$'\033[32m'; C_YELLOW=$'\033[33m'; C_RED=$'\033[31m'; C_RESET=$'\033[0m'
step()  { printf '\n%s== %s ==%s\n' "$C_BOLD" "$1" "$C_RESET"; }
info()  { printf '  %s%s%s\n' "$C_DIM" "$1" "$C_RESET"; }
ok()    { printf '  %s✓%s %s\n' "$C_GREEN" "$C_RESET" "$1"; }
warn()  { printf '  %s!%s %s\n' "$C_YELLOW" "$C_RESET" "$1"; }
fail()  { printf '  %s✗%s %s\n' "$C_RED" "$C_RESET" "$1"; }

for bin in kubectl curl jq; do
  command -v "$bin" >/dev/null || { fail "$bin is required"; exit 1; }
done

PF_PID=""
cleanup() { [ -n "${PF_PID:-}" ] && kill "$PF_PID" 2>/dev/null || true; }
trap cleanup EXIT

step "Port-forwarding $OS_SERVICE -> localhost:$LOCAL_PORT"
kubectl port-forward -n "$NS" "$OS_SERVICE" "$LOCAL_PORT:9200" >/dev/null 2>&1 &
PF_PID=$!
PF_READY=0
for _ in $(seq 1 30); do
  if curl -sS -k "https://localhost:$LOCAL_PORT" >/dev/null 2>&1; then PF_READY=1; break; fi
  sleep 1
done
[ "$PF_READY" -eq 1 ] || { fail "port-forward to $OS_SERVICE never became ready"; exit 1; }
ok "tunnel ready (pid $PF_PID)"

OS_USER=$(kubectl get secret -n "$NS" "$OS_SECRET" -o jsonpath='{.data.username}' | base64 -d)
OS_PASS=$(kubectl get secret -n "$NS" "$OS_SECRET" -o jsonpath='{.data.password}' | base64 -d)
BASE="https://localhost:$LOCAL_PORT"
OS() { curl -sS -k -u "$OS_USER:$OS_PASS" -H 'Content-Type: application/json' "$@"; }

exists()      { [ "$(curl -sS -k -o /dev/null -w '%{http_code}' -u "$OS_USER:$OS_PASS" -I "$BASE/$1")" = "200" ]; }
labels_type() { OS "$BASE/$1/_mapping" | jq -r 'to_entries[0].value.mappings.properties.kubernetes.properties.labels.type // "object"'; }
doc_count()   { OS -XPOST "$BASE/$1/_refresh" >/dev/null; OS "$BASE/$1/_count" | jq -r '.count'; }

expect_ack() {
  local what="$1" resp="$2"
  if [ "$(echo "$resp" | jq -r '.acknowledged // false')" != "true" ]; then
    fail "$what failed: $resp"
    exit 1
  fi
}

# Starts a reindex as a task and polls it, so a long copy does not hang on one HTTP call.
reindex() {
  local src="$1" dst="$2" resp task
  resp=$(OS -XPOST "$BASE/_reindex?wait_for_completion=false&slices=auto" \
    -d "{\"conflicts\":\"proceed\",\"source\":{\"index\":\"$src\"},\"dest\":{\"index\":\"$dst\",\"op_type\":\"create\"}}")
  task=$(echo "$resp" | jq -r '.task // empty')
  [ -n "$task" ] || { fail "could not start reindex $src -> $dst: $resp"; exit 1; }
  info "reindex $src -> $dst (task $task)"
  while :; do
    resp=$(OS "$BASE/_tasks/$task")
    [ "$(echo "$resp" | jq -r '.completed')" = "true" ] && break
    sleep 5
  done
  if [ "$(echo "$resp" | jq '(.error != null) or ((.response.failures // []) | length > 0)')" = "true" ]; then
    fail "reindex $src -> $dst failed: $(echo "$resp" | jq -c '.error // .response.failures[:2]')"
    exit 1
  fi
}

# Recreates the index from the 0.6.0 template. Fluent Bit may create it first with a late
# record; that is fine as long as the template gave it the new mapping.
recreate() {
  local idx="$1" resp
  resp=$(OS -XPUT "$BASE/$idx")
  if [ "$(echo "$resp" | jq -r '.error.type // empty')" = "resource_already_exists_exception" ]; then
    info "$idx was recreated by an incoming write"
  else
    expect_ack "creating $idx" "$resp"
  fi
  [ "$(labels_type "$idx")" = "flat_object" ] || { fail "$idx was not created with the 0.6.0 mapping"; exit 1; }
}

copy_back() {
  local idx="$1" tmp="$2" expected
  expected=$(doc_count "$tmp")
  reindex "$tmp" "$idx"
  local got; got=$(doc_count "$idx")
  if [ "$got" -lt "$expected" ]; then
    fail "$idx has $got documents, $tmp has $expected; keeping $tmp, re-run to resume"
    exit 1
  fi
  expect_ack "deleting $tmp" "$(OS -XDELETE "$BASE/$tmp")"
  ok "$idx rebuilt ($got documents)"
}

step "Checking the $TEMPLATE_NAME index template"
TEMPLATE=$(OS "$BASE/_index_template/$TEMPLATE_NAME" | jq -c '.index_templates[0].index_template.template // empty')
if [ "$(echo "$TEMPLATE" | jq -r '.mappings.properties.kubernetes.properties.labels.type // empty' 2>/dev/null)" != "flat_object" ]; then
  fail "template $TEMPLATE_NAME does not map labels as flat_object"
  info "run the 0.6.0 helm upgrade first and wait for the opensearch setup job to complete"
  exit 1
fi
ok "0.6.0 template is registered"
TMP_BODY=$(echo "$TEMPLATE" | jq -c '{mappings, settings: {number_of_shards: (.settings.index.number_of_shards // .settings.number_of_shards // "1"), number_of_replicas: "0"}}')

TODAY="${INDEX_PREFIX}$(date -u +%Y-%m-%d)"

step "Listing indices"
NAMES=$(
  {
    OS "$BASE/_cat/indices/${INDEX_PREFIX}*?h=index&expand_wildcards=open"
    OS "$BASE/_cat/indices/${TMP_PREFIX}${INDEX_PREFIX}*?h=index&expand_wildcards=open" | sed "s/^${TMP_PREFIX}//"
  } | tr -d '\r' | awk 'NF' | sort -u
)
if [ -z "$NAMES" ]; then
  warn "no indices matched ${INDEX_PREFIX}*, nothing to do"
  exit 0
fi
info "$(echo "$NAMES" | wc -l | tr -d ' ') index/indices found"

MIGRATED=0; SKIPPED=0; DEFERRED=0
while IFS= read -r IDX; do
  TMP="${TMP_PREFIX}${IDX}"
  step "$IDX"

  if exists "$IDX"; then
    if [ "$(labels_type "$IDX")" = "flat_object" ]; then
      if exists "$TMP"; then
        info "resuming: copying documents back from $TMP"
        copy_back "$IDX" "$TMP"
        MIGRATED=$((MIGRATED + 1))
      else
        info "already on the 0.6.0 mapping, skipping"
        SKIPPED=$((SKIPPED + 1))
      fi
      continue
    fi

    if [ "$IDX" = "$TODAY" ]; then
      warn "Fluent Bit is still writing to today's index; re-run tomorrow to migrate it"
      DEFERRED=$((DEFERRED + 1))
      continue
    fi

    if exists "$TMP"; then
      info "discarding partial copy $TMP from an interrupted run"
      expect_ack "deleting $TMP" "$(OS -XDELETE "$BASE/$TMP")"
    fi

    expect_ack "blocking writes on $IDX" \
      "$(OS -XPUT "$BASE/$IDX/_settings" -d '{"index.blocks.write": true}')"
    BEFORE=$(doc_count "$IDX")

    expect_ack "creating $TMP" "$(OS -XPUT "$BASE/$TMP" -d "$TMP_BODY")"
    reindex "$IDX" "$TMP"
    COPIED=$(doc_count "$TMP")
    if [ "$COPIED" -ne "$BEFORE" ]; then
      fail "$TMP has $COPIED documents, $IDX has $BEFORE; $IDX was not modified beyond a write block"
      info "remove it with: PUT $IDX/_settings {\"index.blocks.write\": null}"
      exit 1
    fi
    ok "copied $COPIED documents to $TMP"

    expect_ack "deleting $IDX" "$(OS -XDELETE "$BASE/$IDX")"
  else
    info "resuming: $IDX was deleted by an interrupted run, restoring from $TMP"
  fi

  recreate "$IDX"
  copy_back "$IDX" "$TMP"
  MIGRATED=$((MIGRATED + 1))
done <<< "$NAMES"

step "Done"
ok "$MIGRATED migrated, $SKIPPED already up to date"
if [ "$DEFERRED" -gt 0 ]; then
  warn "$DEFERRED index deferred (today's); run this script again tomorrow"
fi
