# One-command wrkflw demo for Windows. Installs everything, creates the
# database, seeds a demo login with sample content, and starts the server.
#
# Run in PowerShell (it will ask for admin approval for installs only):
#   irm https://raw.githubusercontent.com/utachicodes/wrkflw/main/scripts/setup-demo.ps1 | iex
$ErrorActionPreference = "Stop"

function Write-Step($text) { Write-Host "`n==> $text" -ForegroundColor Cyan }
function Write-Good($text) { Write-Host $text -ForegroundColor Green }

function Test-Admin {
  ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
}

function Ensure-Winget {
  if (Get-Command winget -ErrorAction SilentlyContinue) { return }
  throw "winget was not found. Install 'App Installer' from the Microsoft Store, then run this script again."
}

function Ensure-Package($id, $override) {
  if (winget list --id $id -e 2>$null | Select-String -Quiet $id) { return }
  Write-Step "Installing $id (admin approval needed)..."
  if ($override) { winget install --id $id -e --accept-source-agreements --accept-package-agreements --override $override }
  else { winget install --id $id -e --accept-source-agreements --accept-package-agreements }
}

function Refresh-Path {
  $machine = [Environment]::GetEnvironmentVariable("Path", "Machine")
  $user = [Environment]::GetEnvironmentVariable("Path", "User")
  $env:Path = "$machine;$user"
}

function New-Password($length) {
  $alphabet = "abcdefghijkmnopqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789!-"
  -join (1..$length | ForEach-Object { $alphabet[(Get-Random -Maximum $alphabet.Length)] })
}

# --- Elevate only if installs are missing ---------------------------------
$needInstalls = -not (Get-Command winget -ErrorAction SilentlyContinue) -or
  -not (Get-Command git -ErrorAction SilentlyContinue) -or
  -not (Get-Command go -ErrorAction SilentlyContinue) -or
  -not (Get-Command node -ErrorAction SilentlyContinue) -or
  -not (Test-Path "${env:ProgramFiles}\PostgreSQL\18\bin\psql.exe")
if ($needInstalls -and -not (Test-Admin)) {
  Write-Step "Restarting with admin rights for one-time installs..."
  $command = "irm https://raw.githubusercontent.com/utachicodes/wrkflw/main/scripts/setup-demo.ps1 | iex"
  Start-Process powershell -ArgumentList "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", $command -Verb RunAs -Wait
  exit 0
}

Ensure-Winget
Ensure-Package "Git.Git" ""
Ensure-Package "GoLang.Go" ""
Ensure-Package "OpenJS.NodeJS.LTS" ""
Refresh-Path

$pgBin = "${env:ProgramFiles}\PostgreSQL\18\bin"
$pgPasswordFile = "$env:LOCALAPPDATA\wrkflw\pgpassword.txt"
if (-not (Test-Path "$pgBin\psql.exe")) {
  $pgPassword = New-Password 20
  Ensure-Package "PostgreSQL.18" "--mode unattended --superpassword `"$pgPassword`" --servicename postgresql-x64-18 --servicepassword `"$pgPassword`""
  Refresh-Path
  New-Item -ItemType Directory -Force (Split-Path $pgPasswordFile) | Out-Null
  Set-Content -LiteralPath $pgPasswordFile -Value $pgPassword -NoNewline
} elseif (-not (Test-Path $pgPasswordFile)) {
  throw "PostgreSQL is installed but its password is unknown. Uninstall it (to let this script set a fresh one) or set it up manually per the README."
} else {
  $pgPassword = Get-Content -LiteralPath $pgPasswordFile -Raw
}
$env:PGPASSWORD = $pgPassword
$env:Path = "$pgBin;$env:Path"

# --- Code ------------------------------------------------------------------
$repo = "$HOME\wrkflw"
if (-not (Test-Path "$repo\.git")) {
  Write-Step "Downloading wrkflw..."
  git clone https://github.com/utachicodes/wrkflw.git $repo
}
Set-Location $repo

# --- Database ---------------------------------------------------------------
Write-Step "Creating the database..."
& "$pgBin\createdb.exe" -h 127.0.0.1 -U postgres wrkflw_dev 2>$null
$demoPassword = New-Password 16
$env:DATABASE_URL = "postgres://postgres:$([uri]::EscapeDataString($pgPassword))@127.0.0.1:5432/wrkflw_dev?sslmode=disable"
$env:ADMIN_EMAIL = "demo@wrkflw.local"
$env:ADMIN_PASSWORD = $demoPassword

Write-Step "Migrating and seeding (takes a few minutes on first run)..."
go run ./server/cmd/wrkflw migrate | Select-Object -Last 1
go run ./server/cmd/wrkflw seed-admin | Select-Object -Last 1

# --- Frontend ---------------------------------------------------------------
Write-Step "Building the app (takes a few minutes on first run)..."
npm ci --no-audit --no-fund 2>&1 | Select-Object -Last 1
npm run build:web 2>&1 | Select-Object -Last 1

# --- Sample content ----------------------------------------------------------
Write-Step "Starting the server..."
$env:COOKIE_SECURE = "false"
$env:PORT = "8080"
$server = Start-Process -FilePath "go" -ArgumentList "run", "./server/cmd/wrkflw", "serve" -WorkingDirectory $repo -WindowStyle Hidden -PassThru
for ($i = 0; $i -lt 36; $i++) {
  Start-Sleep -Seconds 5
  try { Invoke-WebRequest -Uri "http://127.0.0.1:8080/api/health" -UseBasicParsing -TimeoutSec 3 | Out-Null; break }
  catch { if ($i -eq 35) { throw "The server did not start. See the background process output." } }
}

$login = @{ email = "demo@wrkflw.local"; password = $demoPassword } | ConvertTo-Json
$null = Invoke-WebRequest -Uri "http://127.0.0.1:8080/api/v1/auth/login" -Method Post -Body $login -ContentType "application/json" -SessionVariable demo -UseBasicParsing
$list = Invoke-WebRequest -Uri "http://127.0.0.1:8080/api/v1/lists" -Method Post -WebSession $demo -Body (@{ name = "Welcome"; goal = "See what wrkflw does in ten seconds" } | ConvertTo-Json) -ContentType "application/json" -UseBasicParsing | ConvertFrom-Json
$tasks = @(
  @{ title = "Chat with your assistant from Telegram"; description = "Connect messaging in Settings, then text your bot. Replies land here." },
  @{ title = "Scan the CLI for security issues"; description = "Run: wrkflw scan --min-severity high"; priority = "p1" }
)
foreach ($task in $tasks) {
  $null = Invoke-WebRequest -Uri "http://127.0.0.1:8080/api/v1/lists/$($list.id)/tasks" -Method Post -WebSession $demo -Body ($task | ConvertTo-Json) -ContentType "application/json" -UseBasicParsing
}
$null = Invoke-WebRequest -Uri "http://127.0.0.1:8080/api/v1/tasks" -Method Post -WebSession $demo -Body (@{ title = "Capture anything from anywhere (inbox)"; description = "Tasks without a list land in your inbox." } | ConvertTo-Json) -ContentType "application/json" -UseBasicParsing
$null = Invoke-WebRequest -Uri "http://127.0.0.1:8080/api/v1/agents" -Method Post -WebSession $demo -Body (@{ displayName = "Scout"; purpose = "Triage inbox captures into clear next actions." } | ConvertTo-Json) -ContentType "application/json" -UseBasicParsing

$card = @"
URL:      http://localhost:8080/login
Email:    demo@wrkflw.local
Password: $demoPassword
"@
Set-Content -LiteralPath "$repo\DEMO-CREDENTIALS.txt" -Value $card

Write-Good "`nDone! Open http://localhost:8080/login"
Write-Host "  Email:    demo@wrkflw.local"
Write-Host "  Password: $demoPassword"
Write-Host "`nThese are also saved in $repo\DEMO-CREDENTIALS.txt"
Write-Host "Stop the server any time from Task Manager (end the go process)."
