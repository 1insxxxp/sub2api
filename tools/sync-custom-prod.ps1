[CmdletBinding()]
param(
    [string]$MainBranch = "main",
    [string]$CustomBranch = "custom-prod",
    [string]$OriginRemote = "origin",
    [string]$UpstreamRemote = "upstream",
    [switch]$Push,
    [switch]$DryRun
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

function Invoke-Git {
    param(
        [Parameter(Mandatory = $true)]
        [string[]]$Args
    )

    $display = "git " + ($Args -join " ")
    Write-Host ">> $display" -ForegroundColor Cyan

    if ($DryRun) {
        return
    }

    & git @Args
    if ($LASTEXITCODE -ne 0) {
        throw "Command failed: $display"
    }
}

function Get-GitOutput {
    param(
        [Parameter(Mandatory = $true)]
        [string[]]$Args
    )

    $output = @(& git @Args)
    if ($LASTEXITCODE -ne 0) {
        throw "Command failed: git $($Args -join ' ')"
    }
    return ,$output
}

function Assert-RemoteExists {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Remote
    )

    $null = Get-GitOutput @("remote", "get-url", $Remote)
}

function Assert-CleanTrackedState {
    $unstaged = Get-GitOutput @("diff", "--name-only")
    $staged = Get-GitOutput @("diff", "--cached", "--name-only")

    if ($unstaged.Count -gt 0 -or $staged.Count -gt 0) {
        throw "Working tree has tracked changes. Please commit or stash them before syncing."
    }
}

function Warn-UntrackedFiles {
    $untracked = Get-GitOutput @("ls-files", "--others", "--exclude-standard")
    if ($untracked.Count -gt 0) {
        Write-Warning "Untracked files will be left untouched:"
        $untracked | ForEach-Object { Write-Warning "  $_" }
    }
}

Assert-RemoteExists -Remote $OriginRemote
Assert-RemoteExists -Remote $UpstreamRemote
Assert-CleanTrackedState
Warn-UntrackedFiles

$currentBranch = (Get-GitOutput @("rev-parse", "--abbrev-ref", "HEAD") | Select-Object -First 1).Trim()
if ($currentBranch -eq "HEAD") {
    throw "Detached HEAD is not supported. Please switch to a branch first."
}

Write-Host "Syncing upstream branch '$MainBranch' into '$CustomBranch'..." -ForegroundColor Green
Invoke-Git @("fetch", $OriginRemote, "--prune")
Invoke-Git @("fetch", $UpstreamRemote, "--prune")

Invoke-Git @("switch", $MainBranch)
Invoke-Git @("merge", "--ff-only", "$UpstreamRemote/$MainBranch")
if ($Push) {
    Invoke-Git @("push", $OriginRemote, $MainBranch)
}

Invoke-Git @("switch", $CustomBranch)
Invoke-Git @("merge", "--no-edit", $MainBranch)
if ($Push) {
    Invoke-Git @("push", $OriginRemote, $CustomBranch)
}

Write-Host "" 
Write-Host "Finished. Current branch: $CustomBranch" -ForegroundColor Green
if (-not $Push) {
    Write-Host "Run again with -Push to publish main/custom-prod to origin." -ForegroundColor Yellow
}
