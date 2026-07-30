@echo off
rem VibePerks launcher invoked by the Claude Code hooks and status line on Windows.
rem A marketplace install ships source (no committed host binary), so we resolve a
rem runnable binary at first use, mirroring the POSIX bin/vibeperks:
rem   1) a built/cached binary (bin\vibeperks.real.exe)
rem   2) a prebuilt distribution binary for this platform (bin\vibeperks-windows-amd64.exe)
rem   3) a prebuilt binary downloaded from the GitHub Release (cached as .real.exe) - no Go
rem   4) build from src\ if Go is available (cached as bin\vibeperks.real.exe)
rem Failures stay quiet (exit 0) so the host CLI is never broken.
setlocal
set "DIR=%~dp0"

if exist "%DIR%vibeperks.real.exe" goto run_real

if exist "%DIR%vibeperks-windows-amd64.exe" (
  "%DIR%vibeperks-windows-amd64.exe" %*
  exit /b
)

rem Download the prebuilt binary from the GitHub Release, cache as vibeperks.real.exe.
rem Channel defaults to the moving "latest" release; set VIBEPERKS_RELEASE_CHANNEL=dev-latest
rem to pull the dev prerelease instead.
set "CHANNEL=%VIBEPERKS_RELEASE_CHANNEL%"
if not defined CHANNEL set "CHANNEL=latest"
if /I "%CHANNEL%"=="latest" (set "URL=https://github.com/VibePerks/claude-code/releases/latest/download/vibeperks-windows-amd64.exe") else (set "URL=https://github.com/VibePerks/claude-code/releases/download/%CHANNEL%/vibeperks-windows-amd64.exe")
where curl >nul 2>nul
if not errorlevel 1 (
  curl -fsSL "%URL%" -o "%DIR%vibeperks.real.exe.tmp" >nul 2>nul
) else (
  powershell -NoProfile -Command "try { Invoke-WebRequest -UseBasicParsing '%URL%' -OutFile '%DIR%vibeperks.real.exe.tmp' } catch { exit 1 }" >nul 2>nul
)
if exist "%DIR%vibeperks.real.exe.tmp" (
  move /y "%DIR%vibeperks.real.exe.tmp" "%DIR%vibeperks.real.exe" >nul 2>nul
  if exist "%DIR%vibeperks.real.exe" goto run_real
)

where go >nul 2>nul
if not errorlevel 1 (
  go build -C "%DIR%..\src" -trimpath -o "%DIR%vibeperks.real.exe" . >nul 2>nul
  if exist "%DIR%vibeperks.real.exe" goto run_real
)

exit /b 0

:run_real
"%DIR%vibeperks.real.exe" %*
exit /b
