#Requires -RunAsAdministrator

param
(
    $Path = "$env:ProgramFiles\telepresence"
)

$current_directory = (Get-Location).path

echo "Installing telepresence to $Path"

Start-Process msiexec -Wait -verb runAs -Args "/i $current_directory\winfsp.msi /passive /qn /L*V winfsp-install.log"
Start-Process msiexec -Wait -verb runAs -Args "/i $current_directory\sshfs-win.msi /passive /qn /L*V sshfs-win-install.log"

if(!(test-path $Path))
{
    New-Item -ItemType Directory -Force -Path $Path
}

Copy-Item "telepresence.exe" -Destination "$Path" -Force
Copy-Item "wintun.dll" -Destination "$Path" -Force

# Update PATH if entries do not exist only
$currentPath = [Environment]::GetEnvironmentVariable("Path", "Machine")
@("$Path", "C:\Program Files\SSHFS-Win\bin") | Where-Object { $currentPath -notlike "*$_*" } | ForEach-Object { $currentPath = "$_;$currentPath" }
[Environment]::SetEnvironmentVariable("Path", $currentPath, "Machine")

echo "Telepresence installed to $Path"