@echo off
setlocal
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%~dp0install-go.ps1" %*
set "SCRIPT_EXIT=%ERRORLEVEL%"
echo.
if not "%SCRIPT_EXIT%"=="0" (
  echo Installation or build failed with exit code %SCRIPT_EXIT%.
) else (
  echo Installation and build completed.
)
pause
exit /b %SCRIPT_EXIT%
