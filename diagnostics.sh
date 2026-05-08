#!/bin/bash
# WarpLocal Diagnostics
#
# Generates a diagnostic report for bug reporting.
# All API keys, tokens, emails, and home directory paths are automatically redacted.
#
# Usage:
#   ./diagnostics.sh
#   bash <(curl -fsSL https://raw.githubusercontent.com/sasuke39/open-warp/main/diagnostics.sh)

set -euo pipefail

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

info()  { echo -e "${GREEN}[INFO]${NC} $*"; }
warn()  { echo -e "${YELLOW}[WARN]${NC} $*"; }
error() { echo -e "${RED}[ERROR]${NC} $*"; exit 1; }

SUPPORT_DIR="$HOME/Library/Application Support/WarpLocal"

redact() {
    sed \
        -e 's/\(api_key:\s*\).*/\1[REDACTED]/gI' \
        -e 's/\(api_key":\s*"\)[^"]*/\1[REDACTED]/gI' \
        -e 's/\(Authorization:\s*Bearer\s*\).*/\1[REDACTED]/gI' \
        -e 's/sk-[a-zA-Z0-9_-]\{8,\}/[REDACTED_API_KEY]/g' \
        -e "s|/Users/[^/)|\\ ]\{1,\}|/Users/[USER]|g" \
        -e 's/[a-zA-Z0-9._%+-]\+@[a-zA-Z0-9.-]\+\.[a-zA-Z]\{2,\}/[REDACTED_EMAIL]/g' \
        -e 's/\(token=\)[^& ]\{1,\}/\1[REDACTED]/g' \
        -e 's/\(password:\s*\).*/\1[REDACTED]/gI'
}

ts="$(date +%Y%m%d-%H%M%S)"
out_dir="$HOME/Desktop/WarpLocal-Diagnostics-${ts}"
mkdir -p "$out_dir"

info "WarpLocal Diagnostics"
info "Output: $out_dir"
echo ""

# ── System info ──
os_ver="$(sw_vers -productVersion 2>/dev/null || echo unknown)"
arch="$(uname -m)"
cpu_type="$(sysctl -n machdep.cpu.brand_string 2>/dev/null | sed 's/ .*//' || echo unknown)"
if [[ "$arch" == "arm64" ]]; then cpu_type="Apple Silicon"; fi
locale_str="$(defaults read -g AppleLocale 2>/dev/null || echo unknown)"

info "System: macOS $os_ver ($arch) $cpu_type Locale=$locale_str"

# ── App info ──
app_path="/Applications/WarpLocal.app"
app_exists="false"
app_quarantine="false"
app_version="unknown"
app_commit="unknown"
app_build_time="unknown"
app_build_arch="unknown"

if [[ -d "$app_path" ]]; then
    app_exists="true"
    if xattr "$app_path" 2>/dev/null | grep -q com.apple.quarantine; then
        app_quarantine="true"
    fi
    build_info="$app_path/Contents/Resources/build-info.json"
    if [[ -f "$build_info" ]]; then
        app_version="$(python3 -c "import json,sys; d=json.load(open('$build_info')); print(d.get('version','unknown'))" 2>/dev/null || echo unknown)"
        app_commit="$(python3 -c "import json,sys; d=json.load(open('$build_info')); print(d.get('git_commit','unknown'))" 2>/dev/null || echo unknown)"
        app_build_time="$(python3 -c "import json,sys; d=json.load(open('$build_info')); print(d.get('build_time','unknown'))" 2>/dev/null || echo unknown)"
        app_build_arch="$(python3 -c "import json,sys; d=json.load(open('$build_info')); print(d.get('build_arch','unknown'))" 2>/dev/null || echo unknown)"
    fi
fi

info "App: exists=$app_exists quarantine=$app_quarantine version=$app_version commit=$app_commit arch=$app_build_arch"

# ── Config info ──
config_path="$SUPPORT_DIR/config.yaml"
config_exists="false"
config_valid="false"
provider="unknown"
base_url_host="unknown"
model="unknown"
missing_fields=()

if [[ -f "$config_path" ]]; then
    config_exists="true"
    provider="$(python3 -c "
import yaml,sys
try:
    d=yaml.safe_load(open('$config_path'))
    print(d.get('provider','unknown'))
except: print('unknown')
" 2>/dev/null || echo unknown)"
    base_url_host="$(python3 -c "
import yaml,sys
try:
    from urllib.parse import urlparse
    d=yaml.safe_load(open('$config_path'))
    u=d.get('base_url','')
    print(urlparse(u).hostname or 'unknown')
except: print('unknown')
" 2>/dev/null || echo unknown)"
    model="$(python3 -c "
import yaml,sys
try:
    d=yaml.safe_load(open('$config_path'))
    print(d.get('model','unknown'))
except: print('unknown')
" 2>/dev/null || echo unknown)"
    for field in provider base_url api_key model; do
        val="$(python3 -c "
import yaml,sys
try:
    d=yaml.safe_load(open('$config_path'))
    v=d.get('$field','')
    print('ok' if v and v.strip() else 'missing')
except: print('missing')
" 2>/dev/null || echo missing)"
        if [[ "$val" == "missing" ]]; then
            missing_fields+=("$field")
        fi
    done
    if [[ ${#missing_fields[@]} -eq 0 ]]; then
        config_valid="true"
    fi
fi

info "Config: exists=$config_exists valid=$config_valid provider=$provider model=$model"
if [[ ${#missing_fields[@]} -gt 0 ]]; then
    warn "Missing config fields: ${missing_fields[*]}"
fi

# ── Helper health ──
helper_port=18888
helper_health="unknown"

if command -v curl &>/dev/null; then
    if curl -s --max-time 3 "http://127.0.0.1:$helper_port/health" 2>/dev/null | python3 -c "import json,sys; d=json.load(sys.stdin); sys.exit(0 if d.get('ok') else 1)" 2>/dev/null; then
        helper_health="ok"
    else
        helper_health="unreachable_or_error"
    fi
fi
info "Helper: health=$helper_health"

# ── Log ──
log_path="$SUPPORT_DIR/warplocal.log"
log_included="false"
log_lines=0

if [[ -f "$log_path" ]]; then
    log_lines="$(wc -l < "$log_path" | tr -d ' ')"
    tail -300 "$log_path" | redact > "$out_dir/warplocal-log-tail.txt"
    log_included="true"
    info "Log: $log_lines lines (last 300 exported, redacted)"
else
    info "Log: not found"
fi

# ── Crash reports ──
crash_included="false"
crash_count=0
latest_crash_time="none"
crash_dir="$HOME/Library/Logs/DiagnosticReports"

if [[ -d "$crash_dir" ]]; then
    crash_files=()
    for f in "$crash_dir"/WarpLocal*.crash "$crash_dir"/WarpLocal*.ips "$crash_dir"/warp*.crash; do
        [[ -f "$f" ]] && crash_files+=("$f")
    done
    crash_count=${#crash_files[@]}
    if [[ $crash_count -gt 0 ]]; then
        {
            for f in "${crash_files[@]}"; do
                echo "================================================================"
                echo "File: $(basename "$f")"
                echo "Modified: $(stat -f '%Sm' -t '%Y-%m-%dT%H:%M:%S' "$f" 2>/dev/null || echo unknown)"
                echo "================================================================"
                awk '
                    /^Process:/ || /^Path:/ || /^Identifier:/ || /^Version:/ || /^Code Type:/ || /^Parent Process:/ || /^Date\/Time:/ || /^OS Version:/ || /^Report Version:/ { print }
                    /^Exception Type:/ || /^Exception Codes:/ || /^Termination/ { print; found=1 }
                    /^Crashed Thread:/ { print; found=1 }
                    /^Thread [0-9]+ Crashed:/ { found=1 }
                    found && /^$/ { found=0 }
                    found { print }
                    /^Binary Images:/ { exit }
                ' "$f"
                echo ""
            done
        } | redact > "$out_dir/crash-report.txt"
        crash_included="true"
        latest_crash_time="$(stat -f '%Sm' -t '%Y-%m-%dT%H:%M:%S' "${crash_files[-1]}" 2>/dev/null || echo unknown)"
        info "Crash reports: $crash_count found (key segments exported, redacted)"
    else
        info "Crash reports: none found"
    fi
else
    info "Crash reports: directory not found"
fi

# ── Write diagnostics.json ──
missing_json="[]"
if [[ ${#missing_fields[@]} -gt 0 ]]; then
    missing_json="$(printf '%s\n' "${missing_fields[@]}" | python3 -c "import json,sys; print(json.dumps(sys.stdin.read().splitlines()))" 2>/dev/null || echo '[]')"
fi

cat > "$out_dir/diagnostics.json" <<DIAGNOSTICS
{
  "schema_version": "1.0",
  "generated_at": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
  "app": {
    "name": "WarpLocal",
    "version": "$app_version",
    "git_commit": "$app_commit",
    "build_time": "$app_build_time",
    "build_arch": "$app_build_arch",
    "install_path": "$app_path",
    "exists": $app_exists,
    "quarantine": $app_quarantine
  },
  "system": {
    "os": "macOS",
    "os_version": "$os_ver",
    "kernel": "$(uname -r)",
    "arch": "$arch",
    "cpu": "$cpu_type",
    "locale": "$locale_str"
  },
  "runtime": {
    "helper_port": $helper_port,
    "helper_health": "$helper_health",
    "config_exists": $config_exists,
    "config_valid": $config_valid,
    "missing_config_fields": $missing_json,
    "provider": "$provider",
    "base_url_host": "$base_url_host",
    "model": "$model"
  },
  "logs": {
    "warplocal_log_included": $log_included,
    "warplocal_log_lines": $log_lines,
    "crash_report_included": $crash_included,
    "crash_report_count": $crash_count,
    "latest_crash_report_time": "$latest_crash_time"
  },
  "redaction": {
    "api_keys_redacted": true,
    "authorization_headers_redacted": true,
    "home_directory_redacted": true,
    "emails_redacted": true,
    "url_query_redacted": true
  }
}
DIAGNOSTICS

# ── Write issue-summary.md ──
missing_text="none"
if [[ ${#missing_fields[@]} -gt 0 ]]; then
    missing_text="${missing_fields[*]}"
fi

cat > "$out_dir/issue-summary.md" <<SUMMARY
## WarpLocal Diagnostics Summary

**Generated:** $(date -u +%Y-%m-%dT%H:%M:%SZ)

### System
- **OS:** macOS $os_ver ($(uname -r))
- **Arch:** $arch ($cpu_type)
- **Locale:** $locale_str

### App
- **Installed:** $app_exists
- **Version:** $app_version (commit: $app_commit, arch: $app_build_arch)
- **Build time:** $app_build_time
- **Quarantine:** $app_quarantine

### Configuration
- **Config exists:** $config_exists
- **Config valid:** $config_valid
- **Missing fields:** $missing_text
- **Provider:** $provider
- **Base URL host:** $base_url_host
- **Model:** $model

### Runtime
- **Helper health:** $helper_health (port $helper_port)

### Logs
- **Log lines:** $log_lines (last 300 exported)
- **Crash reports:** $crash_count (latest: $latest_crash_time)

### Redaction
API keys, tokens, emails, and home directory paths have been redacted.
Please review before posting. Do not add secrets back.

---

*Paste this summary into a [bug report](https://github.com/sasuke39/open-warp/issues/new?template=bug_report.yml). Attach the full diagnostics folder if possible.*
SUMMARY

# ── Write README.txt ──
cat > "$out_dir/README.txt" <<README
WarpLocal Diagnostics
=====================

This folder contains diagnostic information for bug reporting.

Files:
  diagnostics.json       — Machine-readable metadata (safe to upload)
  issue-summary.md       — Copy-paste this into your GitHub issue
  warplocal-log-tail.txt — Last 300 lines of the adapter log (redacted)
  crash-report.txt       — Key segments from macOS crash reports (redacted)

Privacy:
  - API keys, tokens, and passwords have been replaced with [REDACTED]
  - Home directory paths have been replaced with /Users/[USER]
  - Email addresses have been replaced with [REDACTED_EMAIL]

  Please review files before uploading. Do NOT add secrets back.

How to report:
  1. Open https://github.com/sasuke39/open-warp/issues/new?template=bug_report.yml
  2. Copy the contents of issue-summary.md into the "Diagnostics summary" field
  3. Optionally attach this entire folder as a zip
README

echo ""
info "Diagnostics written to: $out_dir"
echo ""
echo "Files:"
ls -1 "$out_dir"
echo ""
info "Next: open the issue template and paste the summary:"
info "  https://github.com/sasuke39/open-warp/issues/new?template=bug_report.yml"
