; =============================================================================
; HashVerifier — Inno Setup Script
; =============================================================================
; Cross-platform checksum generation and verification tool
; =============================================================================

#define MyAppName "HashVerifier"
#define MyAppPublisher "Ostap Konstantinov"
#define MyAppURL "https://github.com/ostapkonst/HashVerifier"
#define MyAppExeName "hashverifier.exe"

; Version passed from command line: /DAppVersion=1.0.0
#ifndef AppVersion
  #error AppVersion is not set. Pass /DAppVersion=X.X.X to the compiler.
#endif

; Architecture passed from command line: /DAppArch=amd64 or /DAppArch=i686
#ifndef AppArch
  #define AppArch "amd64"
#endif

; Source directory passed from command line: /DSourceDir=../dist/windows-amd64
; When running in Docker, this should be set to /work (the mounted directory)
#ifndef SourceDir
  #define SourceDir "."
#endif

[Setup]
; Application identity
AppId={{49028601-93D0-4249-B2C5-ACAA48680F84}
AppName={#MyAppName}
AppVersion={#AppVersion}
AppVerName={#MyAppName} v{#AppVersion} ({#AppArch})
UninstallDisplayName={#MyAppName} ({#AppArch})
AppPublisher={#MyAppPublisher}
AppPublisherURL={#MyAppURL}
AppSupportURL={#MyAppURL}
AppUpdatesURL={#MyAppURL}

; License file
LicenseFile=hashverifier-license.txt

; Installation paths
DefaultDirName={commonpf}\{#MyAppName}
DefaultGroupName={#MyAppName}
AllowNoIcons=yes

; Architecture detection
#if AppArch == "amd64"
  ArchitecturesAllowed=x64compatible
  ArchitecturesInstallIn64BitMode=x64compatible
#else
  ArchitecturesAllowed=x86
#endif

; Output
OutputDir=output
OutputBaseFilename=hashverifier-{#AppVersion}-windows-{#AppArch}
SetupIconFile=favicon.ico
UninstallDisplayIcon={app}\{#MyAppExeName}

; File version information
VersionInfoVersion=1.0.0.0
VersionInfoCompany={#MyAppPublisher}
VersionInfoDescription={#MyAppName} Installer
VersionInfoCopyright=Copyright © 2026 {#MyAppPublisher}

; Compression
Compression=lzma2/ultra64
SolidCompression=yes
LZMAUseSeparateProcess=yes

; Wizard style
WizardStyle=modern
WizardSizePercent=100,100

; Privileges
PrivilegesRequired=admin

; Minimum Windows version
MinVersion=6.1sp1

; Suppress warning about user areas with admin privileges
; SendTo is intentionally per-user; all other paths use common areas
UsedUserAreasWarning=no

; Prevent multiple instances
SetupMutex=HashVerifierSetupMutex

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"

[Tasks]
Name: "desktopicon"; Description: "{cm:CreateDesktopIcon}"; GroupDescription: "{cm:AdditionalIcons}"
Name: "sendtoicon"; Description: "Add to Send-To menu"; GroupDescription: "Shell Integration:"
Name: "assoc_sfv"; Description: ".sfv (CRC32)"; GroupDescription: "File Associations:"
Name: "assoc_md4"; Description: ".md4 (MD4)"; GroupDescription: "File Associations:"
Name: "assoc_md5"; Description: ".md5 (MD5)"; GroupDescription: "File Associations:"
Name: "assoc_sha1"; Description: ".sha1 (SHA1)"; GroupDescription: "File Associations:"
Name: "assoc_sha256"; Description: ".sha256 (SHA256)"; GroupDescription: "File Associations:"
Name: "assoc_sha384"; Description: ".sha384 (SHA384)"; GroupDescription: "File Associations:"
Name: "assoc_sha512"; Description: ".sha512 (SHA512)"; GroupDescription: "File Associations:"
Name: "assoc_sha3_256"; Description: ".sha3-256 (SHA3-256)"; GroupDescription: "File Associations:"
Name: "assoc_sha3_384"; Description: ".sha3-384 (SHA3-384)"; GroupDescription: "File Associations:"
Name: "assoc_sha3_512"; Description: ".sha3-512 (SHA3-512)"; GroupDescription: "File Associations:"
Name: "assoc_blake3"; Description: ".blake3 (BLAKE3)"; GroupDescription: "File Associations:"
Name: "assoc_xxh3"; Description: ".xxh3 (XXH3)"; GroupDescription: "File Associations:"
Name: "assoc_xxh128"; Description: ".xxh128 (XXH128)"; GroupDescription: "File Associations:"

[Files]
; Main executable
Source: "{#SourceDir}\{#MyAppExeName}"; DestDir: "{app}"; Flags: ignoreversion

; GTK3 and runtime DLLs
Source: "{#SourceDir}\*.dll"; DestDir: "{app}"; Flags: ignoreversion recursesubdirs

; GTK3 configuration
Source: "{#SourceDir}\etc\*"; DestDir: "{app}\etc"; Flags: ignoreversion recursesubdirs

; GTK3 schemas
Source: "{#SourceDir}\share\glib-2.0\schemas\*"; DestDir: "{app}\share\glib-2.0\schemas"; Flags: ignoreversion recursesubdirs

; GTK3 icons
Source: "{#SourceDir}\share\icons\*"; DestDir: "{app}\share\icons"; Flags: ignoreversion recursesubdirs

; GTK3 themes (may be empty)
Source: "{#SourceDir}\share\themes\*"; DestDir: "{app}\share\themes"; Flags: ignoreversion recursesubdirs skipifsourcedoesntexist

; Documentation
Source: "{#SourceDir}\LICENSE"; DestDir: "{app}"; Flags: ignoreversion
Source: "{#SourceDir}\THIRD_PARTY_NOTICES"; DestDir: "{app}"; Flags: ignoreversion

; File type icon for associations (installed but not shown in UI)
Source: "hashverifier-filetype.ico"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
; Start Menu (common = all users)
Name: "{commonprograms}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"
Name: "{commonprograms}\{cm:UninstallProgram,{#MyAppName}}"; Filename: "{uninstallexe}"

; Desktop (common = all users)
Name: "{commondesktop}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"; Tasks: desktopicon

; Send-To menu (current user only)
Name: "{userappdata}\Microsoft\Windows\SendTo\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"; Tasks: sendtoicon

[Registry]
; File associations — SFV
Root: HKCR; Subkey: ".sfv"; ValueType: string; ValueName: ""; ValueData: "HashVerifier.SFV"; Flags: uninsdeletevalue; Tasks: assoc_sfv
Root: HKCR; Subkey: "HashVerifier.SFV"; ValueType: string; ValueName: ""; ValueData: "SFV Checksum File"; Flags: uninsdeletekey; Tasks: assoc_sfv
Root: HKCR; Subkey: "HashVerifier.SFV\DefaultIcon"; ValueType: string; ValueName: ""; ValueData: "{app}\hashverifier-filetype.ico"; Tasks: assoc_sfv
Root: HKCR; Subkey: "HashVerifier.SFV\shell\open\command"; ValueType: string; ValueName: ""; ValueData: """{app}\{#MyAppExeName}"" ""%1"""; Tasks: assoc_sfv

; File associations — MD4
Root: HKCR; Subkey: ".md4"; ValueType: string; ValueName: ""; ValueData: "HashVerifier.MD4"; Flags: uninsdeletevalue; Tasks: assoc_md4
Root: HKCR; Subkey: "HashVerifier.MD4"; ValueType: string; ValueName: ""; ValueData: "MD4 Checksum File"; Flags: uninsdeletekey; Tasks: assoc_md4
Root: HKCR; Subkey: "HashVerifier.MD4\DefaultIcon"; ValueType: string; ValueName: ""; ValueData: "{app}\hashverifier-filetype.ico"; Tasks: assoc_md4
Root: HKCR; Subkey: "HashVerifier.MD4\shell\open\command"; ValueType: string; ValueName: ""; ValueData: """{app}\{#MyAppExeName}"" ""%1"""; Tasks: assoc_md4

; File associations — MD5
Root: HKCR; Subkey: ".md5"; ValueType: string; ValueName: ""; ValueData: "HashVerifier.MD5"; Flags: uninsdeletevalue; Tasks: assoc_md5
Root: HKCR; Subkey: "HashVerifier.MD5"; ValueType: string; ValueName: ""; ValueData: "MD5 Checksum File"; Flags: uninsdeletekey; Tasks: assoc_md5
Root: HKCR; Subkey: "HashVerifier.MD5\DefaultIcon"; ValueType: string; ValueName: ""; ValueData: "{app}\hashverifier-filetype.ico"; Tasks: assoc_md5
Root: HKCR; Subkey: "HashVerifier.MD5\shell\open\command"; ValueType: string; ValueName: ""; ValueData: """{app}\{#MyAppExeName}"" ""%1"""; Tasks: assoc_md5

; File associations — SHA1
Root: HKCR; Subkey: ".sha1"; ValueType: string; ValueName: ""; ValueData: "HashVerifier.SHA1"; Flags: uninsdeletevalue; Tasks: assoc_sha1
Root: HKCR; Subkey: "HashVerifier.SHA1"; ValueType: string; ValueName: ""; ValueData: "SHA1 Checksum File"; Flags: uninsdeletekey; Tasks: assoc_sha1
Root: HKCR; Subkey: "HashVerifier.SHA1\DefaultIcon"; ValueType: string; ValueName: ""; ValueData: "{app}\hashverifier-filetype.ico"; Tasks: assoc_sha1
Root: HKCR; Subkey: "HashVerifier.SHA1\shell\open\command"; ValueType: string; ValueName: ""; ValueData: """{app}\{#MyAppExeName}"" ""%1"""; Tasks: assoc_sha1

; File associations — SHA256
Root: HKCR; Subkey: ".sha256"; ValueType: string; ValueName: ""; ValueData: "HashVerifier.SHA256"; Flags: uninsdeletevalue; Tasks: assoc_sha256
Root: HKCR; Subkey: "HashVerifier.SHA256"; ValueType: string; ValueName: ""; ValueData: "SHA256 Checksum File"; Flags: uninsdeletekey; Tasks: assoc_sha256
Root: HKCR; Subkey: "HashVerifier.SHA256\DefaultIcon"; ValueType: string; ValueName: ""; ValueData: "{app}\hashverifier-filetype.ico"; Tasks: assoc_sha256
Root: HKCR; Subkey: "HashVerifier.SHA256\shell\open\command"; ValueType: string; ValueName: ""; ValueData: """{app}\{#MyAppExeName}"" ""%1"""; Tasks: assoc_sha256

; File associations — SHA384
Root: HKCR; Subkey: ".sha384"; ValueType: string; ValueName: ""; ValueData: "HashVerifier.SHA384"; Flags: uninsdeletevalue; Tasks: assoc_sha384
Root: HKCR; Subkey: "HashVerifier.SHA384"; ValueType: string; ValueName: ""; ValueData: "SHA384 Checksum File"; Flags: uninsdeletekey; Tasks: assoc_sha384
Root: HKCR; Subkey: "HashVerifier.SHA384\DefaultIcon"; ValueType: string; ValueName: ""; ValueData: "{app}\hashverifier-filetype.ico"; Tasks: assoc_sha384
Root: HKCR; Subkey: "HashVerifier.SHA384\shell\open\command"; ValueType: string; ValueName: ""; ValueData: """{app}\{#MyAppExeName}"" ""%1"""; Tasks: assoc_sha384

; File associations — SHA512
Root: HKCR; Subkey: ".sha512"; ValueType: string; ValueName: ""; ValueData: "HashVerifier.SHA512"; Flags: uninsdeletevalue; Tasks: assoc_sha512
Root: HKCR; Subkey: "HashVerifier.SHA512"; ValueType: string; ValueName: ""; ValueData: "SHA512 Checksum File"; Flags: uninsdeletekey; Tasks: assoc_sha512
Root: HKCR; Subkey: "HashVerifier.SHA512\DefaultIcon"; ValueType: string; ValueName: ""; ValueData: "{app}\hashverifier-filetype.ico"; Tasks: assoc_sha512
Root: HKCR; Subkey: "HashVerifier.SHA512\shell\open\command"; ValueType: string; ValueName: ""; ValueData: """{app}\{#MyAppExeName}"" ""%1"""; Tasks: assoc_sha512

; File associations — SHA3-256
Root: HKCR; Subkey: ".sha3-256"; ValueType: string; ValueName: ""; ValueData: "HashVerifier.SHA3_256"; Flags: uninsdeletevalue; Tasks: assoc_sha3_256
Root: HKCR; Subkey: "HashVerifier.SHA3_256"; ValueType: string; ValueName: ""; ValueData: "SHA3-256 Checksum File"; Flags: uninsdeletekey; Tasks: assoc_sha3_256
Root: HKCR; Subkey: "HashVerifier.SHA3_256\DefaultIcon"; ValueType: string; ValueName: ""; ValueData: "{app}\hashverifier-filetype.ico"; Tasks: assoc_sha3_256
Root: HKCR; Subkey: "HashVerifier.SHA3_256\shell\open\command"; ValueType: string; ValueName: ""; ValueData: """{app}\{#MyAppExeName}"" ""%1"""; Tasks: assoc_sha3_256

; File associations — SHA3-384
Root: HKCR; Subkey: ".sha3-384"; ValueType: string; ValueName: ""; ValueData: "HashVerifier.SHA3_384"; Flags: uninsdeletevalue; Tasks: assoc_sha3_384
Root: HKCR; Subkey: "HashVerifier.SHA3_384"; ValueType: string; ValueName: ""; ValueData: "SHA3-384 Checksum File"; Flags: uninsdeletekey; Tasks: assoc_sha3_384
Root: HKCR; Subkey: "HashVerifier.SHA3_384\DefaultIcon"; ValueType: string; ValueName: ""; ValueData: "{app}\hashverifier-filetype.ico"; Tasks: assoc_sha3_384
Root: HKCR; Subkey: "HashVerifier.SHA3_384\shell\open\command"; ValueType: string; ValueName: ""; ValueData: """{app}\{#MyAppExeName}"" ""%1"""; Tasks: assoc_sha3_384

; File associations — SHA3-512
Root: HKCR; Subkey: ".sha3-512"; ValueType: string; ValueName: ""; ValueData: "HashVerifier.SHA3_512"; Flags: uninsdeletevalue; Tasks: assoc_sha3_512
Root: HKCR; Subkey: "HashVerifier.SHA3_512"; ValueType: string; ValueName: ""; ValueData: "SHA3-512 Checksum File"; Flags: uninsdeletekey; Tasks: assoc_sha3_512
Root: HKCR; Subkey: "HashVerifier.SHA3_512\DefaultIcon"; ValueType: string; ValueName: ""; ValueData: "{app}\hashverifier-filetype.ico"; Tasks: assoc_sha3_512
Root: HKCR; Subkey: "HashVerifier.SHA3_512\shell\open\command"; ValueType: string; ValueName: ""; ValueData: """{app}\{#MyAppExeName}"" ""%1"""; Tasks: assoc_sha3_512

; File associations — BLAKE3
Root: HKCR; Subkey: ".blake3"; ValueType: string; ValueName: ""; ValueData: "HashVerifier.BLAKE3"; Flags: uninsdeletevalue; Tasks: assoc_blake3
Root: HKCR; Subkey: "HashVerifier.BLAKE3"; ValueType: string; ValueName: ""; ValueData: "BLAKE3 Checksum File"; Flags: uninsdeletekey; Tasks: assoc_blake3
Root: HKCR; Subkey: "HashVerifier.BLAKE3\DefaultIcon"; ValueType: string; ValueName: ""; ValueData: "{app}\hashverifier-filetype.ico"; Tasks: assoc_blake3
Root: HKCR; Subkey: "HashVerifier.BLAKE3\shell\open\command"; ValueType: string; ValueName: ""; ValueData: """{app}\{#MyAppExeName}"" ""%1"""; Tasks: assoc_blake3

; File associations — XXH3
Root: HKCR; Subkey: ".xxh3"; ValueType: string; ValueName: ""; ValueData: "HashVerifier.XXH3"; Flags: uninsdeletevalue; Tasks: assoc_xxh3
Root: HKCR; Subkey: "HashVerifier.XXH3"; ValueType: string; ValueName: ""; ValueData: "XXH3 Checksum File"; Flags: uninsdeletekey; Tasks: assoc_xxh3
Root: HKCR; Subkey: "HashVerifier.XXH3\DefaultIcon"; ValueType: string; ValueName: ""; ValueData: "{app}\hashverifier-filetype.ico"; Tasks: assoc_xxh3
Root: HKCR; Subkey: "HashVerifier.XXH3\shell\open\command"; ValueType: string; ValueName: ""; ValueData: """{app}\{#MyAppExeName}"" ""%1"""; Tasks: assoc_xxh3

; File associations — XXH128
Root: HKCR; Subkey: ".xxh128"; ValueType: string; ValueName: ""; ValueData: "HashVerifier.XXH128"; Flags: uninsdeletevalue; Tasks: assoc_xxh128
Root: HKCR; Subkey: "HashVerifier.XXH128"; ValueType: string; ValueName: ""; ValueData: "XXH128 Checksum File"; Flags: uninsdeletekey; Tasks: assoc_xxh128
Root: HKCR; Subkey: "HashVerifier.XXH128\DefaultIcon"; ValueType: string; ValueName: ""; ValueData: "{app}\hashverifier-filetype.ico"; Tasks: assoc_xxh128
Root: HKCR; Subkey: "HashVerifier.XXH128\shell\open\command"; ValueType: string; ValueName: ""; ValueData: """{app}\{#MyAppExeName}"" ""%1"""; Tasks: assoc_xxh128

[Run]
Filename: "{app}\{#MyAppExeName}"; Description: "{cm:LaunchProgram,{#StringChange(MyAppName, '&', '&&')}}"; Flags: nowait postinstall skipifsilent unchecked

[Code]
procedure InitializeWizard;
begin
  WizardForm.LicenseMemo.WordWrap := False;
  WizardForm.LicenseMemo.ScrollBars := ssBoth;
end;
