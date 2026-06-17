#Requires -Version 5.1
<#
.SYNOPSIS
  Archcore Send — skill client (PowerShell). Mirror of send.sh.

.DESCRIPTION
  Orchestrates `age` + .NET (gzip/sha256) + Invoke-WebRequest to package, encrypt,
  upload, and load end-to-end-encrypted session context. Performs crypto, transport,
  secret scanning, size checks, and temp-file hygiene only — never summarizes, reads
  arbitrary repo files, or mutates project files.

  Output discipline: exactly one JSON object on stdout; all human text on stderr.
  Subcommands / flags / JSON / exit codes mirror send.sh (skill-contract.spec).
#>

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
try { [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12 } catch {}

# --------------------------------------------------------------------------
# Constants (size-limits.rule — canonical; do not redefine elsewhere)
# --------------------------------------------------------------------------
$Script:SendVersion   = 'send.v1'
$Script:DefaultTtl    = '24h'
$Script:MaxTtlSeconds = 7 * 24 * 3600
$Script:CompactSoft   = 30 * 1024
$Script:CompactHard   = 50 * 1024
$Script:EvidenceSoft  = 300 * 1024
$Script:EvidenceHard  = 800 * 1024
$Script:DetailHard    = 50 * 1024 * 1024
$Script:TotalEncSoft  = 10 * 1024 * 1024
$Script:TotalEncHard  = 25 * 1024 * 1024
$Script:GrantDir      = Join-Path ([IO.Path]::GetTempPath()) 'archcore-send-grants'
$Script:GrantWindow   = 540

# --------------------------------------------------------------------------
# Logging (human → stderr)
# --------------------------------------------------------------------------
$Esc = [char]27
function Write-Info    { param($m) [Console]::Error.WriteLine("$Esc[36m$m$Esc[0m") }
function Write-Success { param($m) [Console]::Error.WriteLine("$Esc[32m$m$Esc[0m") }
function Write-WarnMsg  { param($m) [Console]::Error.WriteLine("$Esc[33m$m$Esc[0m") }

# --------------------------------------------------------------------------
# Result + error emission (stdout = one JSON object)
# --------------------------------------------------------------------------
function Emit-Json { param([Parameter(Mandatory)]$Obj)
  [Console]::Out.WriteLine(($Obj | ConvertTo-Json -Compress -Depth 8))
}

function Emit-Error { param($Code, $Message, $Remediation, [int]$ExitCode)
  Emit-Json ([pscustomobject]@{ ok = $false; error_code = $Code; message = $Message; remediation = $Remediation })
  exit $ExitCode
}

# --------------------------------------------------------------------------
# Temp hygiene
# --------------------------------------------------------------------------
$Script:Tmp = $null
function New-Tmp {
  $Script:Tmp = Join-Path ([IO.Path]::GetTempPath()) ("archcore-send." + [Guid]::NewGuid().ToString('N').Substring(0,12))
  New-Item -ItemType Directory -Path $Script:Tmp -Force | Out-Null
}
function Remove-Tmp { if ($Script:Tmp -and (Test-Path $Script:Tmp)) { Remove-Item -Recurse -Force $Script:Tmp -ErrorAction SilentlyContinue } }

# --------------------------------------------------------------------------
# Dependencies
# --------------------------------------------------------------------------
function Test-Cmd { param($Name) [bool](Get-Command $Name -ErrorAction SilentlyContinue) }

function Require-Age {
  if (-not (Test-Cmd 'age')) {
    Emit-Error 'AGE_NOT_FOUND' 'age not found on PATH' `
      'Windows: winget install FiloSottile.age | macOS: brew install age | Linux: distro package' 3
  }
  if (-not (Test-Cmd 'age-keygen')) {
    Emit-Error 'AGE_NOT_FOUND' 'age-keygen not found on PATH' 'install age (provides age-keygen)' 3
  }
}

# --------------------------------------------------------------------------
# Crypto / hashing / compression primitives (.NET — no gzip binary needed)
# --------------------------------------------------------------------------
function Get-Sha256Hex { param($Path) (Get-FileHash -Algorithm SHA256 -Path $Path).Hash.ToLower() }

function Compress-File { param($InPath, $OutPath)
  $in  = [IO.File]::ReadAllBytes($InPath)
  $out = [IO.File]::Create($OutPath)
  try {
    $gz = New-Object IO.Compression.GzipStream($out, [IO.Compression.CompressionMode]::Compress)
    try { $gz.Write($in, 0, $in.Length) } finally { $gz.Dispose() }
  } finally { $out.Dispose() }
}

function Expand-File { param($InPath, $OutPath)
  $in  = [IO.File]::OpenRead($InPath)
  $out = [IO.File]::Create($OutPath)
  try {
    $gz = New-Object IO.Compression.GzipStream($in, [IO.Compression.CompressionMode]::Decompress)
    try { $gz.CopyTo($out) } finally { $gz.Dispose() }
  } finally { $out.Dispose(); $in.Dispose() }
}

function Get-Kind { param($Path)
  switch -Regex ($Path) {
    '\.md$'           { 'markdown'; break }
    '\.(patch|diff)$' { 'patch';    break }
    '\.log$'          { 'log';      break }
    '\.json$'         { 'json';     break }
    '\.txt$'          { 'text';     break }
    default           { 'binary' }
  }
}

function ConvertTo-Seconds { param($Ttl)
  if ($Ttl -notmatch '^[0-9]+[smhd]?$') { return $null }
  $n = [int]($Ttl -replace '[smhd]$', '')
  switch -Regex ($Ttl) {
    's$'        { return $n }
    'm$'        { return $n * 60 }
    'd$'        { return $n * 86400 }
    default     { return $n * 3600 }   # h or bare number
  }
}

# --------------------------------------------------------------------------
# Secret scan (content-policy.rule). Logs counts/types only — never values.
# --------------------------------------------------------------------------
function Invoke-SecretScan { param($Workdir)
  $patterns = @{
    'private-key'    = '-----BEGIN (RSA |EC |OPENSSH |)PRIVATE KEY-----'
    'aws-access-key' = 'AKIA[0-9A-Z]{16}'
    'env-assignment' = '(SECRET|TOKEN|API_KEY|PASSWORD)\s*=\s*\S+'
    'github-token'   = 'ghp_[0-9A-Za-z]{36}|github_pat_[0-9A-Za-z_]{59}'
    'slack-token'    = 'xox[baprs]-[0-9A-Za-z-]+'
    'jwt'            = 'eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+'
    'openai-key'     = 'sk-[A-Za-z0-9]{20,}'
    'db-uri'         = 'postgres://|mysql://|mongodb(\+srv)?://'
  }
  $total = 0
  $files = Get-ChildItem -Path $Workdir -Recurse -File
  foreach ($label in $patterns.Keys) {
    $rx = $patterns[$label]; $hit = 0
    foreach ($f in $files) {
      $content = Get-Content -Raw -ErrorAction SilentlyContinue -Path $f.FullName
      if ($content -and ($content -match $rx)) { $hit++ }
    }
    if ($hit -gt 0) { Write-WarnMsg ("  secret-scan: {0} in {1} file(s)" -f $label, $hit); $total += $hit }
  }
  return $total
}

# --------------------------------------------------------------------------
# Workdir discovery → list of part objects
# --------------------------------------------------------------------------
function Get-Parts { param($Workdir)
  $parts = New-Object System.Collections.ArrayList
  $compact = Join-Path $Workdir 'compact.md'
  if (-not (Test-Path $compact)) {
    Emit-Error 'UNSUPPORTED_VERSION' 'workdir has no compact.md' 'the agent must assemble compact.md before calling send' 2
  }
  $n = 1
  [void]$parts.Add([pscustomobject]@{
    sem='compact'; tid=('part_{0:d4}' -f $n); kind='markdown'; required=$true; lbd=$true
    file=$compact; psize=(Get-Item $compact).Length })

  foreach ($dir in @(@{path='evidence'; req=$true; lbd=$true; pre='evidence'},
                     @{path='details';  req=$false; lbd=$false; pre='detail'})) {
    $d = Join-Path $Workdir $dir.path
    if (Test-Path $d) {
      foreach ($f in (Get-ChildItem -Path $d -File | Sort-Object Name)) {
        $n++
        $stem = [IO.Path]::GetFileNameWithoutExtension($f.Name)
        [void]$parts.Add([pscustomobject]@{
          sem=("{0}.{1}" -f $dir.pre, $stem); tid=('part_{0:d4}' -f $n); kind=(Get-Kind $f.Name)
          required=$dir.req; lbd=$dir.lbd; file=$f.FullName; psize=$f.Length })
      }
    }
  }
  return ,$parts
}

function Test-Sizes { param($Parts, [bool]$IncludeLarge)
  $evid = 0; $soft = $false
  foreach ($p in $Parts) {
    if ($p.sem -eq 'compact') {
      if ($p.psize -gt $Script:CompactHard) {
        Emit-Error 'SEND_TOO_LARGE' ("compact is {0}B (hard cap {1}B)" -f $p.psize,$Script:CompactHard) `
          'split overflow into details/*; compact hard cap is not overridable' 5 }
      if ($p.psize -gt $Script:CompactSoft) { $soft = $true }
    } elseif ($p.sem -like 'evidence.*') {
      $evid += $p.psize
    } elseif ($p.sem -like 'detail.*') {
      if ($p.psize -gt $Script:DetailHard -and -not $IncludeLarge) {
        Emit-Error 'SEND_TOO_LARGE' ("{0} is {1}B (hard cap {2}B)" -f $p.sem,$p.psize,$Script:DetailHard) `
          'drop the detail, or pass -IncludeLarge' 5 }
    }
  }
  if ($evid -gt $Script:EvidenceHard) {
    Emit-Error 'SEND_TOO_LARGE' ("required evidence is {0}B (hard cap {1}B)" -f $evid,$Script:EvidenceHard) `
      'move material into details/*; evidence hard cap is not overridable' 5 }
  if ($evid -gt $Script:EvidenceSoft) { $soft = $true }
  return $soft
}

# --------------------------------------------------------------------------
# Canonical private manifest (matches send.sh layout: one part object per line)
# --------------------------------------------------------------------------
function Write-Manifest { param($Parts, $Title, $OutPath)
  $created = [DateTime]::UtcNow.ToString('yyyy-MM-ddTHH:mm:ssZ')
  $default = ($Parts | Where-Object { $_.lbd } | ForEach-Object { '"' + $_.sem + '"' }) -join ', '
  $lines = @(
    '{'
    '  "version": "' + $Script:SendVersion + '",'
    '  "title": ' + (ConvertTo-Json $Title) + ','
    '  "created_at": "' + $created + '",'
    '  "source": { "agent": "archcore-send-skill" },'
    '  "policy": { "raw_transcript_included": false, "secrets_included": false, "default_load": [' + $default + '] },'
    '  "parts": ['
  )
  for ($i = 0; $i -lt $Parts.Count; $i++) {
    $p = $Parts[$i]
    $obj = '    {"id":"' + $p.sem + '","transport_id":"' + $p.tid + '","kind":"' + $p.kind +
           '","required":' + ($p.required.ToString().ToLower()) + ',"load_by_default":' +
           ($p.lbd.ToString().ToLower()) + ',"plaintext_size":' + $p.psize + '}'
    if ($i -lt $Parts.Count - 1) { $obj += ',' }
    $lines += $obj
  }
  $lines += '  ]'
  $lines += '}'
  Set-Content -Path $OutPath -Value $lines -Encoding UTF8
}

function Get-TitleFromCompact { param($Workdir)
  $first = Get-Content -Path (Join-Path $Workdir 'compact.md') -TotalCount 1
  if ($first -match '^#\s*Context:\s*(.+)$') { return $Matches[1].Trim() }
  return 'Send context'
}

# --------------------------------------------------------------------------
# HTTP helpers (base path only — the #agekey fragment never reaches the network)
# --------------------------------------------------------------------------
function Invoke-Api {
  param($Method, $Url, $Body, $InFile, $OutFile, $Headers)
  $h = @{}
  if ($Headers) { $h = $Headers }
  # The team token gates write endpoints; never clobber a caller-supplied
  # Authorization (the redeem-token Bearer used by part downloads must win).
  if ($env:SEND_TEAM_TOKEN -and -not $h.ContainsKey('Authorization')) {
    $h['Authorization'] = "Bearer $($env:SEND_TEAM_TOKEN)"
  }
  $p = @{ Method = $Method; Uri = $Url; UseBasicParsing = $true; Headers = $h }
  if ($Body)    { $p['Body'] = $Body; $p['ContentType'] = 'application/json' }
  if ($InFile)  { $p['InFile'] = $InFile; $p['ContentType'] = 'application/octet-stream' }
  if ($OutFile) { $p['OutFile'] = $OutFile }
  return Invoke-WebRequest @p
}

# ==========================================================================
# doctor
# ==========================================================================
function Invoke-Doctor { param($Server)
  $ageFound = Test-Cmd 'age'; $ageVer = ''
  if ($ageFound) { try { $ageVer = (& age --version 2>$null | Select-Object -First 1) } catch {} }
  $srvOk = $false
  if ($Server) { try { Invoke-Api 'GET' "$Server/healthz" | Out-Null; $srvOk = $true } catch { $srvOk = $false } }
  Emit-Json ([pscustomobject]@{
    ok = $true
    age = [pscustomobject]@{ found = $ageFound; version = "$ageVer" }
    curl = $true; gzip = $true
    server = [pscustomobject]@{ url = "$Server"; reachable = $srvOk }
  })
}

# ==========================================================================
# send / inspect
# ==========================================================================
function Invoke-Send {
  param($Workdir, [bool]$DryRun, $Opt)
  if (-not (Test-Path $Workdir -PathType Container)) {
    Emit-Error 'UNSUPPORTED_VERSION' "workdir not found: $Workdir" 'pass a valid workdir path' 2 }
  Require-Age
  if (-not $DryRun -and -not $Opt.Server) {
    Emit-Error 'SERVER_UNREACHABLE' 'no server configured' 'set SEND_SERVER_URL or pass -Server URL' 6 }

  $ttlS = ConvertTo-Seconds $Opt.Ttl
  if ($null -eq $ttlS) { Emit-Error 'BAD_REQUEST' "invalid -Ttl: $($Opt.Ttl)" 'use forms like 24h, 7d, 3600s' 2 }
  if ($ttlS -gt $Script:MaxTtlSeconds) { Emit-Error 'BAD_REQUEST' 'ttl exceeds 7d max' 'use -Ttl 7d or less' 2 }

  # 1. Secret scan.
  $hits = Invoke-SecretScan $Workdir
  if ($hits -gt 0 -and -not $Opt.AllowSecrets) {
    Emit-Error 'SECRET_DETECTED' "high-confidence secret(s) detected ($hits file matches)" `
      'redact the secrets, or pass -AllowSecrets to override' 4 }
  if ($hits -gt 0) { Write-WarnMsg "-AllowSecrets: proceeding despite $hits secret match(es)" }

  # 2. Discover + size enforcement.
  $parts = Get-Parts $Workdir
  $softHit = Test-Sizes $parts $Opt.IncludeLarge
  $title = Get-TitleFromCompact $Workdir

  # 3. Manifest.
  $manifest = Join-Path $Script:Tmp 'manifest.json'
  Write-Manifest $parts $title $manifest

  # 4. Preview + confirmation.
  Write-Info "Send preview — `"$title`""
  foreach ($p in $parts) {
    $kb = [Math]::Ceiling($p.psize / 1024)
    if ($p.lbd) { [Console]::Error.WriteLine(("  included : {0,-22} {1}KB" -f $p.sem,$kb)) }
    else        { [Console]::Error.WriteLine(("  optional : {0,-22} {1}KB (lazy detail)" -f $p.sem,$kb)) }
  }
  Write-Info 'The server receives ciphertext + opaque sizes only. The full link (with #agekey) decrypts — treat it like a secret.'
  if ($softHit) { Write-WarnMsg 'Soft size cap exceeded — review before confirming.' }

  $included = @($parts | Where-Object { $_.lbd }     | ForEach-Object { $_.sem })
  $optional = @($parts | Where-Object { -not $_.lbd } | ForEach-Object { $_.sem })

  if ($DryRun) {
    Emit-Json ([pscustomobject]@{ ok=$true; dry_run=$true; one_time=$Opt.OneTime
      included=$included; optional_parts=$optional })
    return
  }

  if (-not $Opt.Yes) {
    $ans = Read-Host 'Proceed with encrypted upload? [y/N]'
    if ($ans -notmatch '^(y|yes)$') { Emit-Error 'BAD_REQUEST' 'upload not confirmed' 're-run with -Yes to skip the prompt' 2 }
  }

  # 5. Ephemeral identity.
  $idfile = Join-Path $Script:Tmp 'id.age'
  & age-keygen -o $idfile 2>$null
  $recipient = (& age-keygen -y $idfile 2>$null)
  $secret = (Get-Content $idfile | Where-Object { $_ -like 'AGE-SECRET-KEY-*' } | Select-Object -First 1)
  if (-not $recipient -or -not $secret) { Emit-Error 'DECRYPTION_FAILED' 'failed to generate age identity' 'check the age installation' 7 }

  # 6. Encrypt (manifest first, then content) → gzip → age.
  $encDir = Join-Path $Script:Tmp 'enc'; New-Item -ItemType Directory -Path $encDir -Force | Out-Null
  $upload = New-Object System.Collections.ArrayList
  $totalEnc = 0
  function Protect-Part { param($Tid, $Src)
    $gz  = Join-Path $encDir "$Tid.gz"
    $enc = Join-Path $encDir "$Tid.age"
    Compress-File $Src $gz
    & age -r $recipient -o $enc $gz
    Remove-Item $gz -Force
    $sz = (Get-Item $enc).Length
    [void]$upload.Add([pscustomobject]@{ tid=$Tid; size=$sz; sha=(Get-Sha256Hex $enc); path=$enc })
    return $sz
  }
  $totalEnc += Protect-Part 'manifest' $manifest
  foreach ($p in $parts) { $totalEnc += Protect-Part $p.tid $p.file }

  if ($totalEnc -gt $Script:TotalEncHard -and -not $Opt.IncludeLarge) {
    Emit-Error 'SEND_TOO_LARGE' ("total encrypted size {0}B (hard cap {1}B)" -f $totalEnc,$Script:TotalEncHard) `
      'drop details, or pass -IncludeLarge' 5 }
  if ($totalEnc -gt $Script:TotalEncSoft) { Write-WarnMsg "Total encrypted size ${totalEnc}B exceeds soft cap." }

  # 7. Create → upload → finalize.
  $partsJson = ($upload | ForEach-Object { '{"part_id":"' + $_.tid + '","encrypted_size":' + $_.size + ',"sha256":"' + $_.sha + '"}' }) -join ','
  $oneTime = $Opt.OneTime.ToString().ToLower()
  $createBody = '{"version":"' + $Script:SendVersion + '","one_time":' + $oneTime + ',"ttl_seconds":' + $ttlS + ',"parts":[' + $partsJson + ']}'
  try { $resp = Invoke-Api 'POST' "$($Opt.Server)/v1/sends" -Body $createBody }
  catch { Emit-Error 'SERVER_UNREACHABLE' 'create request failed' 'check -Server / connectivity' 6 }
  $created = $resp.Content | ConvertFrom-Json
  $sendId = $created.id; $publicUrl = $created.public_url; $expiresAt = $created.expires_at
  if (-not $sendId) { Emit-Error 'STORAGE_ERROR' 'server did not return a send id' 'inspect the server response/logs' 6 }

  foreach ($u in $upload) {
    try { Invoke-Api 'PUT' "$($Opt.Server)/v1/sends/$sendId/parts/$($u.tid)" -InFile $u.path `
            -Headers @{ 'X-Send-Ciphertext-Sha256' = $u.sha } | Out-Null }
    catch { Emit-Error 'SERVER_UNREACHABLE' "upload failed for $($u.tid)" 'retry; check connectivity' 6 }
  }
  try { Invoke-Api 'POST' "$($Opt.Server)/v1/sends/$sendId/finalize" -Body '{}' | Out-Null }
  catch { Emit-Error 'SERVER_UNREACHABLE' 'finalize failed' 'retry; check connectivity' 6 }

  # 8. Append the key locally — never to the network.
  $fullUrl = "$publicUrl#agekey=$secret"
  Write-Success 'Send finalized.'
  Emit-Json ([pscustomobject]@{ ok=$true; url=$fullUrl; expires_at=$expiresAt; one_time=$Opt.OneTime
    included=$included; optional_parts=$optional })
}

# ==========================================================================
# load helpers
# ==========================================================================
function Split-LoadUrl { param($Url, $ServerOverride)
  if ($Url -notmatch '#') { Emit-Error 'FRAGMENT_MISSING' 'URL has no #agekey fragment' 're-copy the full link including everything after #' 7 }
  $base = $Url.Substring(0, $Url.IndexOf('#'))
  $frag = $Url.Substring($Url.IndexOf('#') + 1)
  $key = $null
  if ($frag -like 'agekey=*') { $key = $frag.Substring(7) }
  elseif ($frag -like 'k=*') {
    $b64 = $frag.Substring(2).Replace('_','/').Replace('-','+')
    try { $key = [Text.Encoding]::UTF8.GetString([Convert]::FromBase64String($b64)) } catch { $key = $b64 }
  } else { Emit-Error 'FRAGMENT_MISSING' 'unrecognized fragment encoding' 'expected #agekey=… or #k=…' 7 }
  if (-not $key) { Emit-Error 'FRAGMENT_MISSING' 'empty key in fragment' 're-copy the full link' 7 }
  $id = $base.Substring($base.LastIndexOf('/') + 1)
  $server = if ($ServerOverride) { $ServerOverride } else { $base.Substring(0, $base.IndexOf('/s/')) }
  return [pscustomobject]@{ base=$base; key=$key; id=$id; server=$server }
}

function Get-Grant { param($Ctx)
  New-Item -ItemType Directory -Path $Script:GrantDir -Force | Out-Null
  $gf = Join-Path $Script:GrantDir $Ctx.id
  $now = [int][double]::Parse((Get-Date -UFormat %s))
  if (Test-Path $gf) {
    $cached = Get-Content $gf
    if ($cached.Count -ge 2 -and [int]$cached[0] -gt $now -and $cached[1]) {
      return [pscustomobject]@{ token=$cached[1]; resp=$null }
    }
  }
  try { $r = Invoke-Api 'POST' "$($Ctx.server)/v1/sends/$($Ctx.id)/redeem" -Body '{}' }
  catch {
    $code = 'SERVER_UNREACHABLE'
    try {
      $body = (New-Object IO.StreamReader($_.Exception.Response.GetResponseStream())).ReadToEnd() | ConvertFrom-Json
      if ($body.error_code) { $code = $body.error_code }
    } catch {}
    Emit-Error $code 'redeem rejected' 'the link may be expired or already opened; request a fresh link' 6
  }
  $j = $r.Content | ConvertFrom-Json
  if (-not $j.redeem_token) { Emit-Error 'SERVER_UNREACHABLE' 'no redeem token returned' 'inspect the server response' 6 }
  Set-Content -Path $gf -Value @(($now + $Script:GrantWindow), $j.redeem_token)
  return [pscustomobject]@{ token=$j.redeem_token; resp=$j }
}

function Get-DecryptedPart { param($Ctx, $Tid, $IdFile, $Token)
  $enc = Join-Path $Script:Tmp "dl_$Tid.age"
  $gz  = Join-Path $Script:Tmp "dl_$Tid.gz"
  try { Invoke-Api 'GET' "$($Ctx.server)/v1/sends/$($Ctx.id)/parts/$Tid" -OutFile $enc -Headers @{ Authorization = "Bearer $Token" } | Out-Null }
  catch { Emit-Error 'INTEGRITY_FAILED' "download failed for $Tid" 're-download; the link may be corrupted' 7 }
  $out = $gz + '.out'
  try { & age -d -i $IdFile -o $gz $enc; Expand-File $gz $out }
  catch { Emit-Error 'DECRYPTION_FAILED' "could not decrypt $Tid" 'wrong/truncated key fragment or corrupt download' 7 }
  return (Get-Content -Raw -Path $out)
}

# ==========================================================================
# load
# ==========================================================================
function Invoke-Load { param($Url, $Opt)
  $ctx = Split-LoadUrl $Url $Opt.Server
  Require-Age
  New-Tmp
  $idfile = Join-Path $Script:Tmp 'id.key'
  Set-Content -Path $idfile -Value $ctx.key
  $grant = Get-Grant $ctx

  $manifestPath = Join-Path $Script:Tmp 'manifest.json'
  $manText = Get-DecryptedPart $ctx 'manifest' $idfile $grant.token
  Set-Content -Path $manifestPath -Value $manText
  $man = $manText | ConvertFrom-Json
  if ($man.version -ne 'send.v1') { Emit-Error 'UNSUPPORTED_VERSION' "unsupported format: $($man.version)" 'update the skill' 1 }

  $sizes = @{}
  if ($grant.resp) { foreach ($p in $grant.resp.parts) { $sizes[$p.part_id] = $p.encrypted_size } }

  $compact = ''
  $required = New-Object System.Collections.ArrayList
  $details  = New-Object System.Collections.ArrayList
  foreach ($p in $man.parts) {
    if ($p.id -eq 'compact') {
      $compact = Get-DecryptedPart $ctx $p.transport_id $idfile $grant.token
    } elseif ($p.load_by_default) {
      $content = Get-DecryptedPart $ctx $p.transport_id $idfile $grant.token
      [void]$required.Add([pscustomobject]@{ part_id=$p.id; content=$content })
    } else {
      $sz = if ($sizes.ContainsKey($p.transport_id)) { $sizes[$p.transport_id] } else { 0 }
      [void]$details.Add([pscustomobject]@{ part_id=$p.id; kind=$p.kind; encrypted_size=$sz })
    }
  }
  Write-Success "Loaded `"$($man.title)`" — compact-first. Details listed, not injected."
  Emit-Json ([pscustomobject]@{ ok=$true; title=$man.title; compact_context=$compact
    required_evidence=@($required); available_details=@($details) })
}

# ==========================================================================
# load-detail
# ==========================================================================
function Invoke-LoadDetail { param($Url, $PartId, $Opt)
  $ctx = Split-LoadUrl $Url $Opt.Server
  Require-Age
  New-Tmp
  $idfile = Join-Path $Script:Tmp 'id.key'
  Set-Content -Path $idfile -Value $ctx.key
  $grant = Get-Grant $ctx

  $manText = Get-DecryptedPart $ctx 'manifest' $idfile $grant.token
  $man = $manText | ConvertFrom-Json
  $match = $man.parts | Where-Object { $_.id -eq $PartId } | Select-Object -First 1
  if (-not $match) { Emit-Error 'UNSUPPORTED_VERSION' "no such part: $PartId" 'list parts via --load first' 2 }
  $content = Get-DecryptedPart $ctx $match.transport_id $idfile $grant.token
  Write-Success "Loaded detail `"$PartId`"."
  Emit-Json ([pscustomobject]@{ ok=$true; part_id=$PartId; content=$content })
}

# ==========================================================================
# Argument parsing + dispatch
# ==========================================================================
function Show-Usage {
  [Console]::Error.WriteLine(@'
send.ps1 — Archcore Send skill client

  send.ps1 doctor
  send.ps1 send <workdir> [-Ttl 24h] [-OneTime|-NoOneTime] [-Yes]
                          [-AllowSecrets] [-IncludeLarge] [-DryRun] [-Server URL]
  send.ps1 inspect <workdir>
  send.ps1 load <url> [-Server URL]
  send.ps1 load-detail <url> <part-id>
'@)
}

function Main { param([string[]]$Argv)
  $opt = [pscustomobject]@{
    Server = $env:SEND_SERVER_URL; Ttl = $Script:DefaultTtl; OneTime = $true
    Yes = $false; AllowSecrets = $false; IncludeLarge = $false; DryRun = $false
  }
  if ($Argv.Count -lt 1) { Show-Usage; Emit-Error 'BAD_REQUEST' 'no subcommand' 'see usage above' 2 }
  $sub = $Argv[0]
  $positional = New-Object System.Collections.ArrayList
  for ($i = 1; $i -lt $Argv.Count; $i++) {
    switch -Regex ($Argv[$i]) {
      '^-Ttl$|^--ttl$'                   { $opt.Ttl = $Argv[++$i] }
      '^-OneTime$|^--one-time$'          { $opt.OneTime = $true }
      '^-NoOneTime$|^--no-one-time$'     { $opt.OneTime = $false }
      '^-Yes$|^--yes$'                   { $opt.Yes = $true }
      '^-AllowSecrets$|^--allow-secrets$'{ $opt.AllowSecrets = $true }
      '^-IncludeLarge$|^--include-large$'{ $opt.IncludeLarge = $true }
      '^-DryRun$|^--dry-run$'            { $opt.DryRun = $true }
      '^-Server$|^--server$'             { $opt.Server = $Argv[++$i] }
      '^-h$|^--help$'                    { Show-Usage; exit 0 }
      '^-'                               { Emit-Error 'BAD_REQUEST' "unknown flag: $($Argv[$i])" 'see usage' 2 }
      default                            { [void]$positional.Add($Argv[$i]) }
    }
  }

  switch ($sub) {
    'doctor'       { Invoke-Doctor $opt.Server }
    'send'         { if ($positional.Count -lt 1) { Emit-Error 'BAD_REQUEST' 'send needs a <workdir>' 'send.ps1 send <workdir>' 2 }
                     New-Tmp; Invoke-Send $positional[0] $opt.DryRun $opt }
    'inspect'      { if ($positional.Count -lt 1) { Emit-Error 'BAD_REQUEST' 'inspect needs a <workdir>' 'send.ps1 inspect <workdir>' 2 }
                     New-Tmp; Invoke-Send $positional[0] $true $opt }
    'load'         { if ($positional.Count -lt 1) { Emit-Error 'BAD_REQUEST' 'load needs a <url>' 'send.ps1 load <url>' 2 }
                     Invoke-Load $positional[0] $opt }
    'load-detail'  { if ($positional.Count -lt 2) { Emit-Error 'BAD_REQUEST' 'load-detail needs <url> <part-id>' 'send.ps1 load-detail <url> <part-id>' 2 }
                     Invoke-LoadDetail $positional[0] $positional[1] $opt }
    { $_ -in @('-h','--help','help') } { Show-Usage }
    default        { Show-Usage; Emit-Error 'BAD_REQUEST' "unknown subcommand: $sub" 'see usage above' 2 }
  }
}

try { Main $args }
finally { Remove-Tmp }
